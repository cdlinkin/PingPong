package messages

import (
	"context"
	"fmt"
	"strings"
)

const summaryLimit = 100

const summaryPrompt = `Ты помощник в мессенджере. Тебе дают непрочитанную переписку из чата, каждая строка в формате «логин: сообщение».
Кратко перескажи её на русском языке: от 1 до 5 пунктов, каждый с новой строки и начинается с «- ».
Пиши, кто что предложил, спросил или решил. Не придумывай того, чего нет в переписке. Без вступлений и выводов.`

func (s *Service) SummarizeUnread(
	ctx context.Context,
	userID, chatID int,
) (string, error) {
	if s.summarizer == nil {
		return "", ErrSummaryDisabled
	}

	if err := s.ensureMember(ctx, chatID, userID); err != nil {
		return "", err
	}

	lines, err := s.repository.ListUnreadForSummary(ctx, chatID, userID, summaryLimit)
	if err != nil {
		return "", fmt.Errorf(
			"list unread for summary lines: %w",
			err,
		)
	}

	if len(lines) == 0 {
		return "", ErrNothingToSummarize
	}

	summary, err := s.summarizer.Complete(ctx, summaryPrompt, buildTranscript(lines))
	if err != nil {
		return "", fmt.Errorf(
			"summarize complete: %w",
			err,
		)
	}

	return summary, nil
}

func buildTranscript(lines []SummaryLineModel) string {
	var b strings.Builder

	for _, l := range lines {
		b.WriteString(l.Login)
		b.WriteString(":")
		if l.FileName != nil {
			b.WriteString(" [файл: ")
			b.WriteString(*l.FileName)
			b.WriteString("]")
		}
		if l.Message != "" {
			b.WriteString(" ")
			b.WriteString(l.Message)
		}
		b.WriteString("\n")
	}
	return b.String()
}
