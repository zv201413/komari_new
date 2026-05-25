package update

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/komari-monitor/komari/api"
	"github.com/komari-monitor/komari/database/auditlog"
)

func UploadImage(c *gin.Context) {
	// Limit request body to 10MB
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 10<<20)

	file, header, err := c.Request.FormFile("image")
	if err != nil {
		if strings.Contains(err.Error(), "request body too large") {
			api.RespondError(c, http.StatusRequestEntityTooLarge, "File too large. Maximum size is 10MB")
		} else {
			api.RespondError(c, http.StatusBadRequest, "Invalid upload request: "+err.Error())
		}
		return
	}
	defer file.Close()

	// Read first 512 bytes for MIME validation
	buf := make([]byte, 512)
	n, err := file.Read(buf)
	if err != nil && err != io.EOF {
		api.RespondError(c, http.StatusInternalServerError, "Failed to read file for validation")
		return
	}
	// Rewind file pointer
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		api.RespondError(c, http.StatusInternalServerError, "Failed to reset file pointer")
		return
	}

	contentType := http.DetectContentType(buf[:n])
	
	// SVG needs special handling as DetectContentType might return text/xml
	isSVG := strings.Contains(strings.ToLower(contentType), "xml") && strings.HasSuffix(strings.ToLower(header.Filename), ".svg")
	
	if !strings.HasPrefix(contentType, "image/") && !isSVG {
		api.RespondError(c, http.StatusBadRequest, "Invalid file type. Only images are allowed.")
		return
	}

	// Create uploads directory if not exists
	uploadsDir := "./data/uploads"
	if err := os.MkdirAll(uploadsDir, 0755); err != nil {
		api.RespondError(c, http.StatusInternalServerError, "Failed to create uploads directory")
		return
	}

	// Extract and sanitize extension
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext == "" {
		parts := strings.Split(contentType, "/")
		if len(parts) == 2 {
			ext = "." + parts[1]
		}
	}
	
	if ext != ".jpg" && ext != ".jpeg" && ext != ".png" && ext != ".webp" && ext != ".gif" && ext != ".svg" {
		api.RespondError(c, http.StatusBadRequest, "Unsupported image extension")
		return
	}

	newFilename := fmt.Sprintf("bg_%d%s", time.Now().UnixNano(), ext)
	savePath := filepath.Join(uploadsDir, newFilename)

	out, err := os.Create(savePath)
	if err != nil {
		api.RespondError(c, http.StatusInternalServerError, "Failed to save file: "+err.Error())
		return
	}
	defer out.Close()

	if _, err := io.Copy(out, file); err != nil {
		api.RespondError(c, http.StatusInternalServerError, "Failed to write file")
		return
	}

	uuid, _ := c.Get("uuid")
	auditlog.Log(c.ClientIP(), uuid.(string), "Background image uploaded", "info")

	api.RespondSuccess(c, gin.H{"url": "/uploads/" + newFilename})
}
