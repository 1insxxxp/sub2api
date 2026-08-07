package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type submitEmptyResponseClaimRequest struct {
	Reason string `json:"reason"`
}

func (h *UsageHandler) SubmitEmptyResponseClaim(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	usageLogID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || usageLogID <= 0 {
		response.BadRequest(c, "Invalid usage record id")
		return
	}
	var request submitEmptyResponseClaimRequest
	if err := decodeStrictJSON(c, &request); err != nil {
		response.BadRequest(c, "Invalid claim request")
		return
	}
	if h.emptyResponseClaimService == nil {
		response.InternalError(c, "Empty response claim service not available")
		return
	}
	claim, err := h.emptyResponseClaimService.Submit(c.Request.Context(), service.EmptyResponseClaimSubmitInput{
		UserID: subject.UserID, UsageLogID: usageLogID, UserReason: request.Reason,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	c.JSON(http.StatusCreated, response.Response{Code: 0, Message: "success", Data: dto.EmptyResponseClaimFromService(claim)})
}

func decodeStrictJSON(c *gin.Context, target any) error {
	if c.Request == nil || c.Request.Body == nil {
		return nil
	}
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		if errors.Is(err, io.EOF) {
			return nil
		}
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return errors.New("request body must contain exactly one JSON object")
	}
	return nil
}
