package dto

type CommentEntry struct {
	ID           string `json:"id"`
	AuthorID     string `json:"author_id,omitempty"`
	AuthorName   string `json:"author_name"`
	AuthorRole   string `json:"author_role"`
	AuthorImage  string `json:"author_image,omitempty"`
	Content      string `json:"content"`
	IsInternal   bool   `json:"is_internal"`
	Timestamp    string `json:"timestamp"`
}

type CreateCommentRequest struct {
	AuthorID    string `json:"author_id"`
	AuthorName  string `json:"author_name" validate:"required"`
	AuthorRole  string `json:"author_role"`
	AuthorImage string `json:"author_image"`
	Content     string `json:"content" validate:"required"`
	IsInternal  bool   `json:"is_internal"`
}