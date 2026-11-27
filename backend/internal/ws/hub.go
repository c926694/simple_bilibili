package ws

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	countInterval = 200 * time.Millisecond
	pongWait      = 60 * time.Second
	pingInterval  = 25 * time.Second
	writeTimeout  = 5 * time.Second
	Protocol      = "bilibili"
)

var upgrader = websocket.Upgrader{
	Subprotocols: []string{Protocol},
	CheckOrigin:  sameOrigin,
}

type frame struct {
	Type string `json:"type"`
	Data any    `json:"data"`
}

type peer struct {
	conn    *websocket.Conn
	out     chan []byte
	videoID int64
	userID  int64
}

type room struct {
	peers map[*peer]struct{}
	dirty bool
}

type Hub struct {
	mu      sync.Mutex
	rooms   map[int64]*room
	users   map[int64]map[*peer]struct{}
	closed  bool
	clients sync.WaitGroup
}

func NewHub() *Hub {
	return &Hub{rooms: make(map[int64]*room), users: make(map[int64]map[*peer]struct{})}
}

func (h *Hub) Run(ctx context.Context) {
	ticker := time.NewTicker(countInterval)
	defer ticker.Stop()
	defer h.Close()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			h.broadcastCounts()
		}
	}
}

func (h *Hub) Close() {
	h.mu.Lock()
	h.closed = true
	for _, room := range h.rooms {
		for p := range room.peers {
			h.removeLocked(p)
		}
	}
	h.mu.Unlock()
	h.clients.Wait()
}

func (h *Hub) Serve(w http.ResponseWriter, r *http.Request, videoID, userID int64) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	p := &peer{conn: conn, out: make(chan []byte, 64), videoID: videoID, userID: userID}
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		conn.Close()
		return
	}
	active := h.rooms[videoID]
	if active == nil {
		active = &room{peers: make(map[*peer]struct{})}
		h.rooms[videoID] = active
	}
	active.peers[p] = struct{}{}
	active.dirty = true
	if userID != 0 {
		if h.users[userID] == nil {
			h.users[userID] = make(map[*peer]struct{})
		}
		h.users[userID][p] = struct{}{}
	}
	h.clients.Add(1)
	p.out <- onlineCountFrame(len(active.peers))
	h.mu.Unlock()

	done := make(chan struct{})
	go func() { defer close(done); p.write() }()
	defer func() {
		h.mu.Lock()
		h.removeLocked(p)
		h.mu.Unlock()
		<-done
		h.clients.Done()
	}()
	h.read(p)
}

func (h *Hub) OnlineCount(videoID int64) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	if active := h.rooms[videoID]; active != nil {
		return len(active.peers)
	}
	return 0
}

func (h *Hub) BroadcastVideo(videoID int64, kind string, data any) error {
	payload, err := json.Marshal(frame{kind, data})
	if err != nil {
		return err
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if active := h.rooms[videoID]; active != nil {
		for p := range active.peers {
			h.queueLocked(p, payload)
		}
	}
	return nil
}

func (h *Hub) SendUser(userID int64, kind string, data any) error {
	payload, err := json.Marshal(frame{kind, data})
	if err != nil {
		return err
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for p := range h.users[userID] {
		h.queueLocked(p, payload)
	}
	return nil
}

func (h *Hub) broadcastCounts() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, active := range h.rooms {
		if !active.dirty {
			continue
		}
		active.dirty = false
		payload := onlineCountFrame(len(active.peers))
		for p := range active.peers {
			h.queueLocked(p, payload)
		}
	}
}

func onlineCountFrame(count int) []byte {
	payload, _ := json.Marshal(frame{"online_count", struct {
		Count int `json:"count"`
	}{count}})
	return payload
}

func (h *Hub) queueLocked(p *peer, payload []byte) {
	select {
	case p.out <- payload:
	default:
		// 满队列的连接立即移除，其他订阅者可继续收到广播。
		h.removeLocked(p)
	}
}

func (h *Hub) removeLocked(p *peer) {
	active := h.rooms[p.videoID]
	if active == nil {
		return
	}
	if _, exists := active.peers[p]; !exists {
		return
	}
	delete(active.peers, p)
	active.dirty = true
	if len(active.peers) == 0 {
		delete(h.rooms, p.videoID)
	}
	if p.userID != 0 {
		delete(h.users[p.userID], p)
		if len(h.users[p.userID]) == 0 {
			delete(h.users, p.userID)
		}
	}
	close(p.out)
	p.conn.Close()
}

func (h *Hub) read(p *peer) {
	p.conn.SetReadLimit(4096)
	if err := p.conn.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
		return
	}
	p.conn.SetPongHandler(func(string) error { return p.conn.SetReadDeadline(time.Now().Add(pongWait)) })
	for {
		kind, data, err := p.conn.ReadMessage()
		if err != nil {
			return
		}
		if kind != websocket.TextMessage {
			continue
		}
		var msg struct {
			Type string `json:"type"`
		}
		if json.Unmarshal(data, &msg) != nil || msg.Type != "ping" {
			continue
		}
		h.mu.Lock()
		if active := h.rooms[p.videoID]; active != nil {
			if _, ok := active.peers[p]; ok {
				h.queueLocked(p, []byte(`{"type":"pong","data":{}}`))
			}
		}
		h.mu.Unlock()
	}
}

func (p *peer) write() {
	defer p.conn.Close()
	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()
	for {
		select {
		case payload, ok := <-p.out:
			if !ok {
				return
			}
			if err := p.conn.SetWriteDeadline(time.Now().Add(writeTimeout)); err != nil {
				return
			}
			if err := p.conn.WriteMessage(websocket.TextMessage, payload); err != nil {
				return
			}
		case <-ticker.C:
			if err := p.conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(writeTimeout)); err != nil {
				return
			}
		}
	}
}

func sameOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
		return false
	}
	if strings.EqualFold(u.Host, r.Host) {
		return true
	}
	// 代理使用 Host 的主机名转发时，Origin 仍带浏览器访问端口。
	_, _, portErr := net.SplitHostPort(r.Host)
	return portErr != nil && strings.EqualFold(u.Hostname(), strings.Trim(r.Host, "[]"))
}
