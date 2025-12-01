package ui

import (
	"sync"
	"time"
)

// Task tracking for file downloads
type TaskStatus int

const (
	TaskPending TaskStatus = iota
	TaskActive
	TaskCompleted
	TaskFailed
)

type Task struct {
	ID               string
	FileName         string
	FilePath         []string
	Status           TaskStatus
	TotalBytes       int64
	TransferredBytes int64
	StartTime        time.Time
	EndTime          time.Time
	Speed            float64 // bytes per second
	LastUpdate       time.Time
	LastBytes        int64
	Error            error
	LocalPath        string
}

type TaskManager struct {
	mu    sync.RWMutex
	tasks map[string]*Task
	order []string // chronological order
}

func NewTaskManager() *TaskManager {
	return &TaskManager{
		tasks: make(map[string]*Task),
		order: make([]string, 0),
	}
}

func (tm *TaskManager) Add(task *Task) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.tasks[task.ID] = task
	tm.order = append(tm.order, task.ID)
}

func (tm *TaskManager) Get(id string) *Task {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return tm.tasks[id]
}

func (tm *TaskManager) GetActive() []*Task {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	var active []*Task
	for _, id := range tm.order {
		task := tm.tasks[id]
		if task.Status == TaskActive || task.Status == TaskPending {
			active = append(active, task)
		}
	}
	return active
}

func (tm *TaskManager) GetCompleted(limit int) []*Task {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	var completed []*Task
	// Reverse chronological order
	for i := len(tm.order) - 1; i >= 0 && len(completed) < limit; i-- {
		task := tm.tasks[tm.order[i]]
		if task.Status == TaskCompleted || task.Status == TaskFailed {
			completed = append(completed, task)
		}
	}
	return completed
}
