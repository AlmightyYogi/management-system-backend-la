package utils

import (
	"fmt"
	"io"
	"math/rand"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const MaxFileSize = 10 << 20
const MaxFiles = 5

var AllowedExtensions = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".gif": true,
	".pdf": true, ".doc": true, ".docx": true,
	".xls": true, ".xlsx": true, ".zip": true,
}

func SaveUploadedFiles(c *gin.Context, fieldName string, folder string) ([]string, error) {
	form, err := c.MultipartForm()
	if err != nil {
		return nil, nil
	}

	files := form.File[fieldName]
	if len(files) == 0 {
		return nil, nil
	}
	
	if len(files) > MaxFiles {
		return nil, fmt.Errorf("Maksimal %d file", MaxFiles)
	}

	uploadPath := filepath.Join("storage", "public", folder)
	if err := os.MkdirAll(uploadPath, 0755); err != nil {
		return nil, fmt.Errorf("gagal membuat folder: %v", err)
	}

	var savedFiles []string
	for _, file := range files {
		if file.Size > MaxFileSize {
			return nil, fmt.Errorf("file %s melebihi batas 10MB", file.Filename)
		}

		ext := strings.ToLower(filepath.Ext(file.Filename))
		if !AllowedExtensions[ext] {
			return nil, fmt.Errorf("tipe file %s tidak diizinkan", ext)
		}

		filename := generateFilename(file.Filename)
		dst := filepath.Join(uploadPath, filename)

		if err := c.SaveUploadedFile(file, dst); err != nil {
			return nil, fmt.Errorf("gagal menyimpan file: %v", err)
		}

		savedFiles = append(savedFiles, filename)
	}

	return savedFiles, nil
}

func SaveUploadedFile(c *gin.Context, fieldName string, folder string) (string, error) {
	file, err := c.FormFile(fieldName)
	if err != nil {
		return "", nil
	}

	if file.Size > MaxFileSize {
		return "", fmt.Errorf("file melebihi batas 10MB")
	}

	ext := strings.ToLower(filepath.Ext(file.Filename))
	if !AllowedExtensions[ext] {
		return "", fmt.Errorf("tipe file %s tidak diizinkan", ext)
	}

	uploadPath := filepath.Join("storage", "public", folder)
	if err := os.MkdirAll(uploadPath, 0755); err != nil {
		return "", fmt.Errorf("gagal membuat folder: %v", err)
	}

	filename := generateFilename(file.Filename)
	dst := filepath.Join(uploadPath, filename)
	
	if err := c.SaveUploadedFile(file, dst); err != nil {
		return "", fmt.Errorf("gagal menyimpan file: %v", err)
	}

	return filename, nil
}

func DeleteFiles(folder string, filename string) {
	path := filepath.Join("storage", "public", folder, filename)
	os.Remove(path)
}

func DeleteFilesByFolder(folder string, filenames []string) {
	for _, filename := range filenames {
		DeleteFiles(folder, filename)
	}
}

func generateFilename(original string) string {
	ext := filepath.Ext(original)
	random := randString(8)
	return fmt.Sprintf("%d_%s_%s%s", time.Now().Unix(), random, sanitizeFilename(original[:len(original)-len(ext)]), ext)
}

func sanitizeFilename(name string) string {
	replacer := strings.NewReplacer(" ", "_", "/", "_", "\\", "_")
	return replacer.Replace(name)
}

func randString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func GetExistingFiles(c *gin.Context, fieldName string) []string {
	values := c.PostFormArray(fieldName)
	var result []string
	for _, v := range values {
		if v != "" {
			result = append(result, v)
		}
	}
	return result
}

func FilesToDelete(oldFiles []string, existingFiles []string) []string {
	existingMap := make(map[string]bool)
	for _, f := range existingFiles {
		existingMap[f] = true
	}

	var toDelete []string
	for _, f := range oldFiles {
		if !existingMap[f] {
			toDelete = append(toDelete, f)
		}
	}
	return toDelete
}

func GetMultipartFiles(form *multipart.Form, fieldName string) []*multipart.FileHeader {
	if form == nil {
		return nil
	}
	return form.File[fieldName]
}

func GetMultipartFileHeaders(c *gin.Context, fieldName string) []*multipart.FileHeader {
	form, err := c.MultipartForm()
	if err != nil {
		return nil
	}
	return form.File[fieldName]
}

func SaveFileHeaders(headers []*multipart.FileHeader, folder string) ([]string, error) {
	if len(headers) == 0 {
		return nil, nil
	}

	if len(headers) > MaxFiles {
		return nil, fmt.Errorf("maksimal %d file", MaxFiles)
	}

	uploadPath := filepath.Join("storage", "public", folder)
	if err := os.MkdirAll(uploadPath, 0755); err != nil {
		return nil, fmt.Errorf("gagal membuat folder: %v", err)
	}

	var savedFiles []string
	for _, header := range headers {
		if header.Size > MaxFileSize {
			return nil, fmt.Errorf("file %s melebihi batas 10MB", header.Filename)
		}

		ext := strings.ToLower(filepath.Ext(header.Filename))
		if !AllowedExtensions[ext] {
			return nil, fmt.Errorf("tipe file %s tidak diizinkan", ext)
		}

		src, err := header.Open()
		if err != nil {
			return nil, fmt.Errorf("gagal membuka file: %v", err)
		}
		defer src.Close()

		filename := generateFilename(header.Filename)
		dst := filepath.Join(uploadPath, filename)

		out, err := os.Create(dst)
		if err != nil {
			return nil, fmt.Errorf("gagal membuat file: %v", err)
		}
		defer out.Close()

		if _, err := io.Copy(out, src); err != nil {
			return nil, fmt.Errorf("gagal menyimpan file: %v", err)
		}

		savedFiles = append(savedFiles, filename)
	}

	return savedFiles, nil
}