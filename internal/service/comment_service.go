package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/AlmightyOggy/management-system/internal/dto"
	"github.com/AlmightyOggy/management-system/internal/utils"
)

type CommentService interface {
	GetComments(reportUUID string) ([]dto.CommentEntry, error)
	AddComment(reportUUID string, req dto.CreateCommentRequest) (dto.CommentEntry, error)
	DeleteComment(reportUUID, commentID, requesterID string, isAdmin bool) error
}

type commentService struct {
	baseDir string
	mu      sync.Mutex
}

func NewCommentService(baseDir string) CommentService {
	if baseDir == "" {
		baseDir = "storage/comments"
	}
	_ = os.MkdirAll(baseDir, 0755)
	return &commentService{baseDir: baseDir}
}

func (s *commentService) filePath(reportUUID string) string {
	safeName := filepath.Base(reportUUID) // defensive: prevent path traversal
	return filepath.Join(s.baseDir, fmt.Sprintf("%s.json", safeName))
}

func (s *commentService) GetComments(reportUUID string) ([]dto.CommentEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.readFile(reportUUID)
}

func (s *commentService) readFile(reportUUID string) ([]dto.CommentEntry, error) {
	path := s.filePath(reportUUID)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []dto.CommentEntry{}, nil
		}
		return nil, err
	}
	if len(data) == 0 {
		return []dto.CommentEntry{}, nil
	}
	var comments []dto.CommentEntry
	if err := json.Unmarshal(data, &comments); err != nil {
		return nil, err
	}
	return comments, nil
}

func (s *commentService) writeFile(reportUUID string, comments []dto.CommentEntry) error {
	path := s.filePath(reportUUID)
	data, err := json.MarshalIndent(comments, "", "  ")
	if err != nil {
		return err
	}
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

func (s *commentService) AddComment(reportUUID string, req dto.CreateCommentRequest) (dto.CommentEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	comments, err := s.readFile(reportUUID)
	if err != nil {
		return dto.CommentEntry{}, err
	}

	entry := dto.CommentEntry{
		ID:          utils.GenerateUUID().String(),
		AuthorID:    req.AuthorID,
		AuthorName:  req.AuthorName,
		AuthorRole:  req.AuthorRole,
		AuthorImage: req.AuthorImage,
		Content:     req.Content,
		IsInternal:  req.IsInternal,
		Timestamp:   time.Now().Format("2006-01-02 15:04"),
	}

	comments = append(comments, entry)

	if err := s.writeFile(reportUUID, comments); err != nil {
		return dto.CommentEntry{}, err
	}

	return entry, nil
}

func (s *commentService) DeleteComment(reportUUID, commentID, requesterID string, isAdmin bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	comments, err := s.readFile(reportUUID)
	if err != nil {
		return err
	}

	idx := -1
	for i, c := range comments {
		if c.ID == commentID {
			idx = i
			break
		}
	}
	if idx == -1 {
		return errors.New("comment not found")
	}

	if !isAdmin && comments[idx].AuthorID != requesterID {
		return errors.New("not authorized to delete this comment")
	}

	comments = append(comments[:idx], comments[idx+1:]...)

	return s.writeFile(reportUUID, comments)
}