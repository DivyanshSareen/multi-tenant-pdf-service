package api

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/divyansh/multi-tenant-pdf-service/internal/models"
	"github.com/gin-gonic/gin"
)

// validTenantName allows only alphanumeric characters, hyphens, and underscores.
var validTenantName = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// uploadHandler is the primary endpoint — it accepts a PDF + tenant name,
// provisions the tenant if needed, extracts text, summarizes, stores everything,
// and returns the summary.
//
// POST /api/v1/upload
// Content-Type: multipart/form-data
// Fields: file (PDF), tenantName (string)
func (s *Server) uploadHandler(c *gin.Context) {
	ctx := c.Request.Context()
	log := s.log.WithField("handler", "upload")

	// --- 1. Parse and validate form fields ---
	tenantName := strings.TrimSpace(c.PostForm("tenantName"))
	if tenantName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenantName is required"})
		return
	}
	if !validTenantName.MatchString(tenantName) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "tenantName may only contain letters, numbers, hyphens, and underscores",
		})
		return
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file is required"})
		return
	}

	// Validate content type — accept application/pdf and octet-stream (browser quirk).
	contentType := fileHeader.Header.Get("Content-Type")
	if contentType != "application/pdf" && contentType != "application/octet-stream" {
		if !strings.HasSuffix(strings.ToLower(fileHeader.Filename), ".pdf") {
			c.JSON(http.StatusBadRequest, gin.H{"error": "only PDF files are accepted"})
			return
		}
	}

	// --- 2. Save to temp file ---
	tempDir := os.TempDir()
	tempFileName := fmt.Sprintf("pdf_%d_%s", time.Now().UnixNano(), filepath.Base(fileHeader.Filename))
	tempPath := filepath.Join(tempDir, tempFileName)

	if err := c.SaveUploadedFile(fileHeader, tempPath); err != nil {
		log.WithError(err).Error("saving uploaded file to temp")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save uploaded file"})
		return
	}
	defer os.Remove(tempPath) // always clean up

	log.WithFields(map[string]interface{}{
		"tenant":   tenantName,
		"file":     fileHeader.Filename,
		"size":     fileHeader.Size,
		"tmp_path": tempPath,
	}).Info("received upload request")

	// --- 3. Provision tenant (idempotent — returns existing if already active) ---
	tenant, isNew, err := s.tenantMgr.GetOrCreateTenant(ctx, tenantName)
	if err != nil {
		log.WithError(err).Error("provisioning tenant")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to provision tenant: " + err.Error()})
		return
	}
	log.WithField("is_new_tenant", isNew).Info("tenant ready")

	// --- 4. Extract text from PDF ---
	text, pageCount, err := s.extractor.Extract(tempPath)
	if err != nil {
		log.WithError(err).Error("extracting PDF text")
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "failed to extract text from PDF: " + err.Error()})
		return
	}
	log.WithField("page_count", pageCount).Info("pdf text extracted")

	// --- 5. Summarize via LLM ---
	summary, err := s.summarizer.Summarize(ctx, text)
	if err != nil {
		log.WithError(err).Error("summarizing text")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to summarize document: " + err.Error()})
		return
	}
	log.Info("document summarized")

	// --- 6. Upload original PDF to MinIO ---
	objectName := fmt.Sprintf("%d_%s", time.Now().UnixNano(), fileHeader.Filename)
	fileRef, err := s.store.UploadFile(ctx, tenant.BucketName, objectName, tempPath)
	if err != nil {
		log.WithError(err).Error("uploading to minio")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to store file: " + err.Error()})
		return
	}
	log.WithField("file_ref", fileRef).Info("file stored in minio")

	// --- 7. Persist document metadata + summary to MongoDB ---
	doc := &models.Document{
		FileName:      fileHeader.Filename,
		FullText:      text,
		Summary:       summary,
		FileReference: fileRef,
		FileSize:      fileHeader.Size,
		PageCount:     pageCount,
	}
	docID, err := s.mongo.InsertDocument(ctx, tenant.MongoDBName, doc)
	if err != nil {
		log.WithError(err).Error("inserting document into mongodb")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save document metadata: " + err.Error()})
		return
	}
	log.WithField("document_id", docID).Info("document metadata saved")

	// --- 8. Respond ---
	c.JSON(http.StatusCreated, models.UploadResponse{
		DocumentID:    docID,
		TenantName:    tenantName,
		FileName:      fileHeader.Filename,
		Summary:       summary,
		FileReference: fileRef,
		PageCount:     pageCount,
		IsNewTenant:   isNew,
	})
}
