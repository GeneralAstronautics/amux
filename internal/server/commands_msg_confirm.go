package server

import (
	"fmt"
	"strings"
	"time"

	"github.com/weill-labs/amux/internal/mailbox"
)

const (
	msgConfirmDefaultTimeout = 30 * time.Second
	msgConfirmPollInterval   = 10 * time.Millisecond
)

type msgConfirmStatus string

const (
	msgConfirmNone msgConfirmStatus = ""
	msgConfirmRead msgConfirmStatus = "read"
	msgConfirmAck  msgConfirmStatus = "ack"
)

type msgConfirmOutput struct {
	Status     string             `json:"status"`
	Satisfied  bool               `json:"satisfied"`
	Pending    []string           `json:"pending"`
	Deliveries []msgSummaryOutput `json:"deliveries"`
}

type msgSendConfirmOutput struct {
	msgSendOutput
	Confirm msgConfirmOutput `json:"confirm"`
}

func waitForMsgConfirmation(ctx *CommandContext, id mailbox.MessageID, recipients []mailbox.PaneAddress, status msgConfirmStatus, timeout time.Duration) (msgConfirmOutput, error) {
	deadline := time.Now().Add(timeout)
	for {
		confirm, err := queryMsgConfirmation(ctx, id, recipients, status)
		if err != nil {
			return confirm, err
		}
		if confirm.Satisfied {
			return confirm, nil
		}
		if !time.Now().Before(deadline) {
			return confirm, fmt.Errorf("timed out waiting for %s from %s\n%s", status, strings.Join(confirm.Pending, ","), formatMsgConfirmText(confirm))
		}

		sleep := time.Until(deadline)
		if sleep > msgConfirmPollInterval {
			sleep = msgConfirmPollInterval
		}
		select {
		case <-ctx.context().Done():
			return confirm, ctx.context().Err()
		case <-time.After(sleep):
		}
	}
}

func queryMsgConfirmation(ctx *CommandContext, id mailbox.MessageID, recipients []mailbox.PaneAddress, status msgConfirmStatus) (msgConfirmOutput, error) {
	confirm, err := enqueueSessionQueryOnState(ctx.context(), ctx.Sess, func(sess *Session) (msgConfirmOutput, error) {
		summaries := make([]mailbox.DeliverySummary, 0, len(recipients))
		pending := make([]string, 0)
		store := sess.ensureMailbox()
		for _, recipient := range recipients {
			summary, err := store.DeliverySummary(id, recipient.ID)
			if err != nil {
				return msgConfirmOutput{}, err
			}
			summaries = append(summaries, summary)
			if !msgDeliverySatisfies(summary, status) {
				pending = append(pending, recipient.Name)
			}
		}
		return msgConfirmOutput{
			Status:     string(status),
			Satisfied:  len(pending) == 0,
			Pending:    pending,
			Deliveries: summariesOutput(summaries),
		}, nil
	})
	if err != nil {
		return msgConfirmOutput{}, err
	}
	return confirm, nil
}

func msgDeliverySatisfies(summary mailbox.DeliverySummary, status msgConfirmStatus) bool {
	switch status {
	case msgConfirmRead:
		return !summary.ReadAt.IsZero()
	case msgConfirmAck:
		return !summary.AckedAt.IsZero()
	default:
		return true
	}
}

func formatMsgSendConfirmOutput(msg mailbox.Message, confirm msgConfirmOutput, format msgFormat) (string, error) {
	if format == msgFormatJSON {
		return encodeMsgJSON(msgSendConfirmOutput{
			msgSendOutput: sendOutputForMessage(msg),
			Confirm:       confirm,
		})
	}
	return fmt.Sprintf("Sent %s to %s\n%s", msg.ID, joinPaneNames(msg.Recipients), formatMsgConfirmText(confirm)), nil
}

func formatMsgConfirmText(confirm msgConfirmOutput) string {
	var b strings.Builder
	fmt.Fprintf(&b, "recipient status read_at acked_at ack_status\n")
	for _, delivery := range confirm.Deliveries {
		fmt.Fprintf(&b, "%s %s %s %s %s\n",
			delivery.Recipient.Name,
			msgConfirmDeliveryStatus(delivery),
			msgConfirmValueOrDash(delivery.ReadAt),
			msgConfirmValueOrDash(delivery.AckedAt),
			msgConfirmValueOrDash(delivery.AckStatus))
	}
	return b.String()
}

func msgConfirmDeliveryStatus(delivery msgSummaryOutput) string {
	if delivery.AckedAt != "" {
		return "ack"
	}
	if delivery.ReadAt != "" {
		return "read"
	}
	return "pending"
}

func msgConfirmValueOrDash(value string) string {
	if value == "" {
		return "-"
	}
	return value
}
