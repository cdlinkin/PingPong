package messages

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

func (r *Repository) ListUnreadForSummary(
	ctx context.Context,
	chatID, userID, limit int,
) ([]SummaryLineModel, error) {
	query := `
	SELECT login, message, file_name
	FROM (
	SELECT m.id, u.login, m.message, a.file_name
	FROM messages m
	JOIN chat_members cm ON cm.chat_id = m.chat_id AND cm.user_id = $2
	JOIN users u ON u.id = m.user_id
	LEFT JOIN message_attachments a ON a.message_id = m.id
	WHERE m.chat_id = $1
	AND m.id > cm.last_read_message_id
	AND m.deleted_at IS NULL
	ORDER BY m.id DESC
	LIMIT $3
	) AS last_unread
	ORDER BY id;
	`

	rows, err := r.pool.Query(ctx, query, chatID, userID, limit)
	if err != nil {
		return nil, fmt.Errorf(
			"list unread for summary: %w",
			err,
		)
	}

	lines, err := pgx.CollectRows(rows, pgx.RowToStructByPos[SummaryLineModel])
	if err != nil {
		return nil, fmt.Errorf(
			"collect summary lines: %w",
			err,
		)
	}

	return lines, nil
}
