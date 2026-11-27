package upload

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"simple_bilibili/backend/internal/infra/minio"
	"simple_bilibili/backend/internal/infra/rocketmq"
	"simple_bilibili/backend/internal/modules/video"
	"simple_bilibili/backend/internal/mq"
	"simple_bilibili/backend/internal/pkg/errcode"
	"simple_bilibili/backend/internal/pkg/snowflake"
)

const (
	imageExpiry = 5 * time.Minute
	partExpiry  = 15 * time.Minute
)

type Service struct {
	videos   *video.Service
	store    *Store
	objects  *minio.Client
	producer *rocketmq.Producer
	ids      *snowflake.Generator
}

func NewService(videos *video.Service, store *Store, objects *minio.Client, producer *rocketmq.Producer, ids *snowflake.Generator) *Service {
	return &Service{videos: videos, store: store, objects: objects, producer: producer, ids: ids}
}

func (s *Service) ImagePresign(ctx context.Context, userID int64, prefix, extension, contentType string) (imageResponse, error) {
	key := fmt.Sprintf("%s/%d/%d.%s", prefix, userID, s.ids.Next(), extension)
	url, err := s.objects.PresignPutObject(ctx, key, contentType, imageExpiry)
	if err != nil {
		return imageResponse{}, err
	}
	return imageResponse{
		UploadURL: url, ObjectKey: key, PublicURL: "/storage/" + key,
		Headers: map[string]string{"Content-Type": contentType}, ExpiresIn: int(imageExpiry.Seconds()),
	}, nil
}

func (s *Service) InitVideo(ctx context.Context, userID int64, input videoInput) (initResponse, error) {
	// 先找仍可读取的已完成对象，秒传也为当前用户记录单独的任务。
	key, err := s.reusableKey(ctx, input)
	if err != nil {
		return initResponse{}, err
	}
	record := task{ID: s.ids.Next(), UserID: userID, FileHash: input.FileHash, FileName: input.FileName, FileSize: input.FileSize}
	if key != "" {
		record.ObjectKey, record.Status = key, statusCompleted
		if err := s.store.insert(ctx, record); err != nil {
			return initResponse{}, err
		}
		result := initResult(record, []minio.UploadedPart{})
		result.Instant = true
		return result, nil
	}
	// 续传只使用存储服务提供的实际分片清单。
	if result, found, err := s.resumeVideo(ctx, userID, input); found || err != nil {
		return result, err
	}
	// 新建存储会话，并保存后续查询和合片所需的任务记录。
	return s.newVideoUpload(ctx, record, input)
}

func (s *Service) reusableKey(ctx context.Context, input videoInput) (string, error) {
	keys, err := s.store.completedKeys(ctx, input.FileHash, input.FileSize)
	if err != nil {
		return "", err
	}
	for _, key := range keys {
		info, err := s.objects.StatObject(ctx, key)
		if errors.Is(err, minio.ErrObjectNotFound) {
			continue
		}
		if err != nil {
			return "", err
		}
		if info.Size == input.FileSize {
			return key, nil
		}
	}
	return "", nil
}

func (s *Service) resumeVideo(ctx context.Context, userID int64, input videoInput) (initResponse, bool, error) {
	record, err := s.store.activeTask(ctx, userID, input.FileHash, input.FileSize)
	if errors.Is(err, sql.ErrNoRows) {
		return initResponse{}, false, nil
	}
	if err != nil {
		return initResponse{}, false, err
	}
	parts, err := s.objects.ListObjectParts(ctx, record.ObjectKey, *record.UploadID)
	if errors.Is(err, minio.ErrUploadNotFound) {
		info, statErr := s.objects.StatObject(ctx, record.ObjectKey)
		if statErr == nil && info.Size == record.FileSize {
			// 合片后投递失败时，重新完成同一个任务并返回已上传结果。
			if _, err := s.Complete(ctx, userID, record.ID, nil); err != nil {
				return initResponse{}, false, err
			}
			result := initResult(record, []minio.UploadedPart{})
			result.Instant = true
			result.ChunkSize, result.TotalChunks = 0, 0
			return result, true, nil
		}
		if statErr != nil && !errors.Is(statErr, minio.ErrObjectNotFound) {
			return initResponse{}, false, statErr
		}
		return initResponse{}, false, s.store.setStatus(ctx, record.ID, statusCanceled)
	}
	if err != nil {
		return initResponse{}, false, err
	}
	result := initResult(record, parts)
	result.Resumed = true
	return result, true, nil
}

func (s *Service) newVideoUpload(ctx context.Context, record task, input videoInput) (initResponse, error) {
	record.ChunkSize = chunkSize
	record.TotalChunks = int((record.FileSize + chunkSize - 1) / chunkSize)
	record.ObjectKey = fmt.Sprintf("raw/%d/%d%s", record.UserID, record.ID, input.extension)
	uploadID, err := s.objects.NewMultipartUpload(ctx, record.ObjectKey, input.ContentType)
	if err != nil {
		return initResponse{}, err
	}
	record.UploadID = &uploadID
	if err := s.store.insert(ctx, record); err != nil {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		return initResponse{}, errors.Join(err, s.objects.AbortMultipartUpload(cleanup, record.ObjectKey, uploadID))
	}
	return initResult(record, []minio.UploadedPart{}), nil
}

func initResult(record task, parts []minio.UploadedPart) initResponse {
	return initResponse{TaskID: record.ID, ObjectKey: record.ObjectKey,
		ChunkSize: record.ChunkSize, TotalChunks: record.TotalChunks, UploadedParts: parts}
}

func (s *Service) PartURLs(ctx context.Context, userID, taskID int64, numbers []int) (partURLResponse, error) {
	// 签名前确认调用者拥有该任务且任务处于上传状态。
	record, err := s.ownedTask(ctx, userID, taskID, false)
	if err != nil {
		return partURLResponse{}, err
	}
	if record.Status != statusUploading {
		return partURLResponse{}, errcode.InvalidUploadState
	}
	result := partURLResponse{URLs: make([]partURL, 0, len(numbers)), ExpiresIn: int(partExpiry.Seconds())}
	for _, number := range numbers {
		if number > record.TotalChunks {
			return partURLResponse{}, errcode.InvalidParams
		}
		url, err := s.objects.PresignUploadPart(ctx, record.ObjectKey, *record.UploadID, number, partExpiry)
		if err != nil {
			return partURLResponse{}, err
		}
		result.URLs = append(result.URLs, partURL{PartNumber: number, UploadURL: url})
	}
	return result, nil
}

func (s *Service) Complete(ctx context.Context, userID, taskID int64, submitted []submittedPart) (completeResponse, error) {
	var result completeResponse
	var invalidSize bool
	err := s.store.inTransaction(ctx, func(ctx context.Context) error {
		// 合片与取消共用同一行锁，避免同一个任务同时变更状态。
		record, err := s.ownedTask(ctx, userID, taskID, true)
		if err != nil {
			return err
		}
		if record.Status != statusUploading {
			return errcode.InvalidUploadState
		}
		// 分片清单和合并结果均以 MinIO 返回的信息为准。
		if err := s.mergeParts(ctx, record, submitted); err != nil {
			return err
		}
		info, err := s.objects.StatObject(ctx, record.ObjectKey)
		if errors.Is(err, minio.ErrObjectNotFound) {
			return errcode.InvalidUploadState
		}
		if err != nil {
			return err
		}
		if info.Size != record.FileSize {
			if err := s.objects.RemoveObject(ctx, record.ObjectKey); err != nil {
				return err
			}
			invalidSize = true
			return s.store.setStatus(ctx, record.ID, statusCanceled)
		}
		if err := s.store.setStatus(ctx, record.ID, statusCompleted); err != nil {
			return err
		}
		// 投递失败时回退数据库状态，下次完成请求可复用已合并对象。
		event := mq.VideoTranscodeTask{
			TaskID: record.ID, UserID: record.UserID, ObjectKey: record.ObjectKey,
			FileHash: record.FileHash, FileSize: record.FileSize,
		}
		if _, err := s.producer.PublishJSON(ctx, mq.VideoTranscodeTopic, event, rocketmq.WithKeys(strconv.FormatInt(record.ID, 10))); err != nil {
			return err
		}
		result = completeResponse{TaskID: record.ID, ObjectKey: record.ObjectKey, Status: statusCompleted}
		return nil
	})
	if err != nil {
		return completeResponse{}, err
	}
	if invalidSize {
		return completeResponse{}, errcode.InvalidUploadParts
	}
	return result, nil
}

func (s *Service) mergeParts(ctx context.Context, record task, submitted []submittedPart) error {
	actual, err := s.objects.ListObjectParts(ctx, record.ObjectKey, *record.UploadID)
	if errors.Is(err, minio.ErrUploadNotFound) {
		// 已合并的任务在外层直接检查对象大小。
		return nil
	}
	if err != nil {
		return err
	}
	if len(actual) != record.TotalChunks {
		return errcode.InvalidUploadParts
	}
	slices.SortFunc(actual, func(a, b minio.UploadedPart) int { return a.PartNumber - b.PartNumber })
	parts := make([]minio.CompletePart, len(actual))
	for i, part := range actual {
		expectedSize := min(int64(record.ChunkSize), record.FileSize-int64(i)*int64(record.ChunkSize))
		if part.PartNumber != i+1 || part.Size != expectedSize {
			return errcode.InvalidUploadParts
		}
		parts[i] = minio.CompletePart{PartNumber: part.PartNumber, ETag: part.ETag}
	}
	for _, part := range submitted {
		if part.PartNumber > len(parts) || strings.Trim(part.ETag, "\"") != strings.Trim(parts[part.PartNumber-1].ETag, "\"") {
			return errcode.InvalidUploadParts
		}
	}
	return s.objects.CompleteMultipartUpload(ctx, record.ObjectKey, *record.UploadID, parts)
}

func (s *Service) Abort(ctx context.Context, userID, taskID int64) error {
	return s.store.inTransaction(ctx, func(ctx context.Context) error {
		// 取消前持有同一任务的行锁。
		record, err := s.ownedTask(ctx, userID, taskID, true)
		if err != nil {
			return err
		}
		if record.Status != statusUploading {
			return errcode.InvalidUploadState
		}
		if err := s.objects.AbortMultipartUpload(ctx, record.ObjectKey, *record.UploadID); err != nil && !errors.Is(err, minio.ErrUploadNotFound) {
			return err
		}
		return s.store.setStatus(ctx, taskID, statusCanceled)
	})
}

func (s *Service) Detail(ctx context.Context, userID, taskID int64) (detailResponse, error) {
	// 读取任务归属；只有上传中的任务需要查询分片。
	record, err := s.ownedTask(ctx, userID, taskID, false)
	if err != nil {
		return detailResponse{}, err
	}
	parts := []minio.UploadedPart{}
	if record.Status == statusUploading {
		actual, err := s.objects.ListObjectParts(ctx, record.ObjectKey, *record.UploadID)
		if err == nil {
			parts = actual
		} else if !errors.Is(err, minio.ErrUploadNotFound) {
			return detailResponse{}, err
		}
	}
	return detailResponse{TaskID: record.ID, FileName: record.FileName, FileSize: record.FileSize,
		ChunkSize: record.ChunkSize, TotalChunks: record.TotalChunks, ObjectKey: record.ObjectKey,
		Status: record.Status, UploadedParts: parts}, nil
}

func (s *Service) ownedTask(ctx context.Context, userID, taskID int64, lock bool) (task, error) {
	record, err := s.store.byID(ctx, taskID, lock)
	if errors.Is(err, sql.ErrNoRows) {
		return task{}, errcode.UploadNotFound
	}
	if err != nil {
		return task{}, err
	}
	if record.UserID != userID {
		return task{}, errcode.Forbidden
	}
	return record, nil
}
