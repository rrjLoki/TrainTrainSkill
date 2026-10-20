// Package store 提供仓储层。当前是进程内内存实现，接口形态按可替换为
// SQL 实现来设计。
package store

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sort"
	"strings"
	"sync"
	"time"
)

// ErrNotFound 表示目标记录不存在。
var ErrNotFound = errors.New("item not found")

// Item 是一条待办记录。
type Item struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Done      bool      `json:"done"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Store 是并发安全的内存仓储。
type Store struct {
	mu    sync.RWMutex
	items map[string]Item
	now   func() time.Time
}

// New 创建一个空仓储。
func New() *Store {
	return &Store{
		items: make(map[string]Item),
		now:   time.Now,
	}
}

// List 返回全部记录，按创建时间正序。
func (s *Store) List() []Item {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]Item, 0, len(s.items))
	for _, it := range s.items {
		out = append(out, it)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].ID < out[j].ID
		}
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out
}

// Get 按 ID 取一条记录。
func (s *Store) Get(id string) (Item, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	it, ok := s.items[id]
	if !ok {
		return Item{}, ErrNotFound
	}
	return it, nil
}

// Create 新建一条记录，标题已由调用方校验。
func (s *Store) Create(title string) Item {
	now := s.now()
	it := Item{
		ID:        newID(),
		Title:     strings.TrimSpace(title),
		CreatedAt: now.UTC(),
		UpdatedAt: now.UTC(),
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[it.ID] = it
	return it
}

// Update 局部更新记录，nil 字段表示不改动。
func (s *Store) Update(id string, title *string, done *bool) (Item, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	it, ok := s.items[id]
	if !ok {
		return Item{}, ErrNotFound
	}
	if title != nil {
		it.Title = strings.TrimSpace(*title)
	}
	if done != nil {
		it.Done = *done
	}
	it.UpdatedAt = s.now().UTC()
	s.items[id] = it
	return it, nil
}

// Delete 删除记录，不存在时返回 ErrNotFound。
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.items[id]; !ok {
		return ErrNotFound
	}
	delete(s.items, id)
	return nil
}

func newID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand 失败在实践中意味着系统已不可用，直接 panic 比返回脏数据安全。
		panic("store: cannot generate id: " + err.Error())
	}
	return hex.EncodeToString(b[:])
}
