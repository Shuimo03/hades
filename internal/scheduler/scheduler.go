package scheduler

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

type Job func(ctx context.Context)

// Scheduler manages scheduled jobs
type Scheduler struct {
	cron     *cron.Cron
	jobs     map[string]cron.EntryID
	mu       sync.RWMutex
	callback func(jobName string, result string) // callback for job results
}

// New creates a new scheduler
func New() *Scheduler {
	return &Scheduler{
		cron: cron.New(cron.WithSeconds()),
		jobs: make(map[string]cron.EntryID),
	}
}

// SetCallback sets the callback function for job results
func (s *Scheduler) SetCallback(fn func(jobName string, result string)) {
	s.callback = fn
}

// Start starts the scheduler
func (s *Scheduler) Start() {
	s.cron.Start()
	log.Println("[Scheduler] Started")
}

// Stop stops the scheduler
func (s *Scheduler) Stop() {
	ctx := s.cron.Stop()
	<-ctx.Done()
	log.Println("[Scheduler] Stopped")
}

// AddJob adds a new job with cron expression
func (s *Scheduler) AddJob(name string, spec string, job Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Remove existing job with same name
	if id, ok := s.jobs[name]; ok {
		s.cron.Remove(id)
	}

	id, err := s.cron.AddFunc(spec, func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		log.Printf("[Scheduler] Running job: %s", name)
		result := ""
		func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("[Scheduler] Job %s panicked: %v", name, r)
					result = "panic"
				}
				if s.callback != nil && result != "" {
					s.callback(name, result)
				}
			}()
			job(ctx)
			result = "success"
		}()
	})

	if err != nil {
		return err
	}

	s.jobs[name] = id
	log.Printf("[Scheduler] Added job: %s with schedule: %s", name, spec)
	return nil
}

// AddJobAt adds a job that runs at specific time
func (s *Scheduler) AddJobAt(name string, hour int, minute int, job Job) error {
	spec := fmt.Sprintf("%d %d * * *", minute, hour)
	return s.AddJob(name, spec, job)
}

// RemoveJob removes a job by name
func (s *Scheduler) RemoveJob(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if id, ok := s.jobs[name]; ok {
		s.cron.Remove(id)
		delete(s.jobs, name)
		log.Printf("[Scheduler] Removed job: %s", name)
	}
}

// ListJobs returns all job names
func (s *Scheduler) ListJobs() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	names := make([]string, 0, len(s.jobs))
	for name := range s.jobs {
		names = append(names, name)
	}
	return names
}

// GetNextRun returns the next run time for a job
func (s *Scheduler) GetNextRun(name string) time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if id, ok := s.jobs[name]; ok {
		entry := s.cron.Entry(id)
		return entry.Next
	}
	return time.Time{}
}

// ValidateCronExpr validates a cron expression
func ValidateCronExpr(expr string) error {
	_, err := cron.ParseStandard(expr)
	return err
}
