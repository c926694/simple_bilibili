package errcode

type Code int

const (
	Success         Code = 0
	InvalidParams   Code = 10001
	Unauthorized    Code = 10002
	Forbidden       Code = 10003
	NotFound        Code = 10004
	TooManyRequests Code = 10005
	InternalError   Code = 10500
)

func (c Code) Message() string {
	switch c {
	case Success:
		return "ok"
	case InvalidParams:
		return "参数错误"
	case Unauthorized:
		return "未登录或凭证无效"
	case Forbidden:
		return "无操作权限"
	case NotFound:
		return "资源或接口不存在"
	case TooManyRequests:
		return "请求过于频繁"
	case InternalError:
		return "服务内部错误"
	default:
		return "未知错误"
	}
}
