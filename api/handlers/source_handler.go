package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"reflect"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/google/uuid"
	"go-ai-study/api/fault"
	"go-ai-study/api/models"
	"go-ai-study/api/sourcefiles"
	"go-ai-study/api/utils"
	"go-ai-study/internal/ai"
	"go-ai-study/internal/source"
)

type SourceService interface {
	Scan(context.Context, uint, uint, string) (source.ScanResult, error)
	SubmitAnalyze(context.Context, uint, uint, string) (models.Job, error)
	GetJob(context.Context, uint, string) (models.Job, error)
	SubmitIndex(context.Context, uint, uint, string) (models.Job, error)
	Ask(context.Context, uint, uint, string, string) (*ai.AnswerResult, error)
	CancelJob(context.Context, uint, string) (models.Job, error)
	SubmitLegacy(context.Context, uint, uint, string) (models.Job, error)
}

type SourceHandler struct {
	Service SourceService
	Tokens  *utils.Tokens
}
type targetRequest struct {
	Target string `json:"target"`
}

// decodeRequest rejects unknown fields and multiple JSON values, with bounded
// input and generic errors. Empty operation bodies select the project root.
func decodeRequest(c *gin.Context, target any, allowEmpty bool) error {
	if c.Request.Body == nil {
		if allowEmpty {
			return nil
		}
		return io.EOF
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16<<10)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	err := decoder.Decode(target)
	if allowEmpty && errors.Is(err, io.EOF) {
		return nil
	}
	if err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return errors.New("expected one JSON object")
	}
	value := reflect.ValueOf(target)
	for value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return errors.New("expected one JSON object")
		}
		value = value.Elem()
	}
	return binding.Validator.ValidateStruct(target)
}

func (h *SourceHandler) projectRequest(c *gin.Context) (uint, uint, string, bool) {
	ownerID, id, ok := h.projectIdentity(c)
	if !ok {
		return 0, 0, "", false
	}
	req := &targetRequest{}
	if c.Request.ContentLength != 0 {
		if err := decodeRequest(c, &req, true); err != nil || req == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body; only target is accepted"})
			return 0, 0, "", false
		}
	}
	return ownerID, id, req.Target, true
}

func (h *SourceHandler) projectIdentity(c *gin.Context) (uint, uint, bool) {
	owner, ok := c.Get("user_id")
	ownerID, typed := owner.(uint)
	if !ok || !typed || ownerID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return 0, 0, false
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project id"})
		return 0, 0, false
	}
	if h.Service == nil {
		h.fail(c, fault.ErrStorage)
		return 0, 0, false
	}
	return ownerID, uint(id), true
}

func (h *SourceHandler) Scan(c *gin.Context) {
	owner, project, target, ok := h.projectRequest(c)
	if !ok {
		return
	}
	result, err := h.Service.Scan(c.Request.Context(), owner, project, target)
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *SourceHandler) SourceInfo(c *gin.Context) {
	owner, project, ok := h.projectIdentity(c)
	if !ok {
		return
	}
	provider, ok := h.Service.(interface {
		SourceInfo(context.Context, uint, uint) (sourcefiles.SourceInfo, error)
	})
	if !ok {
		h.fail(c, fault.ErrStorage)
		return
	}
	info, err := provider.SourceInfo(c.Request.Context(), owner, project)
	if err != nil {
		h.fail(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, info)
}

func (h *SourceHandler) Analyze(c *gin.Context) {
	owner, project, target, ok := h.projectRequest(c)
	if !ok {
		return
	}
	job, err := h.Service.SubmitAnalyze(c.Request.Context(), owner, project, target)
	if err != nil {
		h.fail(c, err)
		return
	}
	c.Header("Location", "/api/v1/jobs/"+job.ID)
	c.JSON(http.StatusAccepted, gin.H{"job_id": job.ID, "project_id": job.ProjectID, "status": job.Status})
}

func (h *SourceHandler) GetJob(c *gin.Context) {
	owner, ok := c.Get("user_id")
	ownerID, typed := owner.(uint)
	if !ok || !typed || ownerID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid job id"})
		return
	}
	if h.Service == nil {
		h.fail(c, fault.ErrStorage)
		return
	}
	var job models.Job
	if c.Request.Method == http.MethodPost {
		var req *struct{}
		if err := decodeRequest(c, &req, true); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
			return
		}
		job, err = h.Service.CancelJob(c.Request.Context(), ownerID, id.String())
	} else {
		job, err = h.Service.GetJob(c.Request.Context(), ownerID, id.String())
	}
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, job)
}

func (h *SourceHandler) Index(c *gin.Context) {
	owner, project, target, ok := h.projectRequest(c)
	if !ok {
		return
	}
	job, err := h.Service.SubmitIndex(c.Request.Context(), owner, project, target)
	if err != nil {
		h.fail(c, err)
		return
	}
	c.Header("Location", "/api/v1/jobs/"+job.ID)
	c.JSON(http.StatusAccepted, gin.H{"job_id": job.ID, "project_id": job.ProjectID, "status": job.Status})
}

func (h *SourceHandler) Ask(c *gin.Context) {
	owner, project, ok := h.projectIdentity(c)
	if !ok {
		return
	}
	var req *struct {
		Question string `json:"question"`
		File     string `json:"file"`
	}
	if err := decodeRequest(c, &req, false); err != nil || req == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body; only question and file are accepted"})
		return
	}
	answer, err := h.Service.Ask(c.Request.Context(), owner, project, req.Question, req.File)
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, answer)
}

func (h *SourceHandler) fail(c *gin.Context, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, fault.ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, fault.ErrInvalidTarget):
		status = http.StatusBadRequest
	case errors.Is(err, fault.ErrInvalidQuestion):
		status = http.StatusBadRequest
	case errors.Is(err, fault.ErrProjectBusy):
		status = http.StatusConflict
		c.Header("Retry-After", "1")
	case errors.Is(err, fault.ErrTimeout):
		status = http.StatusGatewayTimeout
	case errors.Is(err, fault.ErrCanceled):
		status = http.StatusRequestTimeout
	case errors.Is(err, fault.ErrAIUnavailable):
		status = http.StatusServiceUnavailable
	case errors.Is(err, fault.ErrAskFailed), errors.Is(err, fault.ErrIndexFailed):
		status = http.StatusBadGateway
	case errors.Is(err, fault.ErrUnsafePath), errors.Is(err, fault.ErrExcludedTarget):
		status = http.StatusForbidden
	case errors.Is(err, fault.ErrSourceUnavailable), errors.Is(err, fault.ErrInvalidSource):
		status = http.StatusUnprocessableEntity
	case errors.Is(err, fault.ErrQueueFull):
		status = http.StatusServiceUnavailable
		c.Header("Retry-After", "1")
	case errors.Is(err, fault.ErrStorage):
		status = http.StatusServiceUnavailable
	case errors.Is(err, fault.ErrStopping):
		status = http.StatusServiceUnavailable
	case errors.Is(err, fault.ErrConflict):
		status = http.StatusConflict
	}
	code, message := fault.Public(err)
	c.JSON(status, gin.H{"error": message, "code": code})
}
