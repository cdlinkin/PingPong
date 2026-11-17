package messages

import (
	"context"
	"errors"
	"net/http"

	"github.com/cdlinkin/pingpong/internal/auth"
	"github.com/cdlinkin/pingpong/internal/httpx"
)

// SummarizeUnread godoc
//
//	@Summary		Краткий пересказ непрочитанного
//	@Description	Просит модель пересказать непрочитанные сообщения чата: до 100 последних, включая свои. Текст пересказа нигде не сохраняется. Если адрес модели не задан, ответ 503. Если вы не участник чата, ответ 404.
//	@Tags			messages
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		int				true	"ID чата"
//	@Success		200	{object}	summaryResponse	"Пересказ"
//	@Failure		400	{string}	string			"invalid chat id"
//	@Failure		401	{string}	string			"invalid token"
//	@Failure		404	{string}	string			"chat not found"
//	@Failure		422	{string}	string			"nothing to summarize"
//	@Failure		502	{string}	string			"summary unavailable"
//	@Failure		503	{string}	string			"summary is disabled"
//	@Failure		504	{string}	string			"summary timed out"
//	@Router			/chats/{id}/summary [post]
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
