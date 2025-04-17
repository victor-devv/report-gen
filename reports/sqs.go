package reports

import "github.com/google/uuid"

type SqsMessage struct {
	UserId   uuid.UUID `json:"user_id"`
	ReportId uuid.UUID `json:"report_id"`
}
