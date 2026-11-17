package messages

import (
	"context"
	"errors"
	"net/http"

	"github.com/cdlinkin/pingpong/internal/auth"
	"github.com/cdlinkin/pingpong/internal/httpx"
)

func (h *Handler) SummarizeUnread(rw http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		http.Error(rw, "unauthorized", http.StatusUnauthorized)
		return
	}

	chatID, err := httpx.PathInt(r, "id")
	if err != nil {
		http.Error(rw, "invalid chat id", http.StatusBadRequest)
		return
	}

	summary, err := h.service.SummarizeUnread(r.Context(), userID, chatID)
	if err != nil {
		switch {
		case errors.Is(err, ErrChatNotFound):
			http.Error(rw, "chat not found", http.StatusNotFound)
		case errors.Is(err, ErrNothingToSummarize):
			http.Error(rw, "nothing to summarize", http.StatusUnprocessableEntity)
		case errors.Is(err, ErrSummaryDisabled):
			http.Error(rw, "summary is disabled", http.StatusServiceUnavailable)
		case errors.Is(err, context.DeadlineExceeded):
			h.logger.Error("summarize unread", "err", err)
			http.Error(rw, "summary timed out", http.StatusGatewayTimeout)
		default:
			h.logger.Error("summarize unread", "err", err)
			http.Error(rw, "summary unavailable", http.StatusBadGateway)
		}
		return
	}

	response := summaryResponse{
		Summary: summary,
	}

	if err := httpx.WriteJSON(rw, http.StatusOK, response); err != nil {
		h.logger.Warn("summarize unread: encode response", "err", err)
	}
}
