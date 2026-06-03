package server

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/weill-labs/amux/internal/mailbox"
	"github.com/weill-labs/amux/internal/mux"
)

type msgSendConfirmTestOutput struct {
	ID      string `json:"id"`
	Confirm struct {
		Status     string   `json:"status"`
		Satisfied  bool     `json:"satisfied"`
		Pending    []string `json:"pending"`
		Deliveries []struct {
			Recipient struct {
				Name string `json:"name"`
			} `json:"recipient"`
			ReadAt    string `json:"read_at"`
			AckedAt   string `json:"acked_at"`
			AckStatus string `json:"ack_status"`
		} `json:"deliveries"`
	} `json:"confirm"`
}

func TestMsgCommandSendWaitAckReportsJSON(t *testing.T) {
	t.Parallel()

	srv, sess, p1, p2, cleanup := newMailboxTestSession(t)
	defer cleanup()
	p3 := addMailboxTestPane(t, sess, 3, "pane-3")

	resultCh := make(chan struct {
		output string
		cmdErr string
	}, 1)
	go func() {
		resultCh <- runTestCommand(t, srv, sess, "msg", "send",
			"--from", p1.Meta.Name,
			"--to", p2.Meta.Name+","+p3.Meta.Name,
			"--subject", "confirm me",
			"--body", "body",
			"--wait-ack",
			"--timeout", "2s",
			"--format", "json")
	}()

	waitForMailboxDelivery(t, sess, "msg-000001", p2.ID)
	waitForMailboxDelivery(t, sess, "msg-000001", p3.ID)
	for _, pane := range []*mux.Pane{p2, p3} {
		read := runTestCommand(t, srv, sess, "msg", "read", "msg-000001", "--for", pane.Meta.Name)
		if read.cmdErr != "" {
			t.Fatalf("msg read %s error = %q", pane.Meta.Name, read.cmdErr)
		}
		ack := runTestCommand(t, srv, sess, "msg", "ack", "msg-000001", "--for", pane.Meta.Name, "--status", "ok")
		if ack.cmdErr != "" {
			t.Fatalf("msg ack %s error = %q", pane.Meta.Name, ack.cmdErr)
		}
	}

	res := readMsgConfirmResult(t, resultCh)
	if res.cmdErr != "" {
		t.Fatalf("msg send --wait-ack error = %q", res.cmdErr)
	}

	var out msgSendConfirmTestOutput
	if err := json.Unmarshal([]byte(res.output), &out); err != nil {
		t.Fatalf("unmarshal msg send --wait-ack output: %v\n%s", err, res.output)
	}
	if out.ID != "msg-000001" || out.Confirm.Status != "ack" || !out.Confirm.Satisfied || len(out.Confirm.Pending) != 0 {
		t.Fatalf("confirm summary = %#v, want ack satisfied with no pending recipients", out)
	}
	got := make(map[string]struct {
		readAt    string
		ackedAt   string
		ackStatus string
	})
	for _, delivery := range out.Confirm.Deliveries {
		got[delivery.Recipient.Name] = struct {
			readAt    string
			ackedAt   string
			ackStatus string
		}{readAt: delivery.ReadAt, ackedAt: delivery.AckedAt, ackStatus: delivery.AckStatus}
	}
	for _, name := range []string{"pane-2", "pane-3"} {
		delivery, ok := got[name]
		if !ok || delivery.readAt == "" || delivery.ackedAt == "" || delivery.ackStatus != "ok" {
			t.Fatalf("delivery[%s] = %#v, want read and ok ack", name, delivery)
		}
	}
}

func TestMsgCommandSendWaitReadCompletesBeforeAck(t *testing.T) {
	t.Parallel()

	srv, sess, p1, p2, cleanup := newMailboxTestSession(t)
	defer cleanup()

	resultCh := make(chan struct {
		output string
		cmdErr string
	}, 1)
	go func() {
		resultCh <- runTestCommand(t, srv, sess, "msg", "send",
			"--from", p1.Meta.Name,
			"--to", p2.Meta.Name,
			"--body", "body",
			"--wait-read",
			"--timeout", "2s",
			"--format", "json")
	}()

	waitForMailboxDelivery(t, sess, "msg-000001", p2.ID)
	read := runTestCommand(t, srv, sess, "msg", "read", "msg-000001", "--for", p2.Meta.Name)
	if read.cmdErr != "" {
		t.Fatalf("msg read error = %q", read.cmdErr)
	}

	res := readMsgConfirmResult(t, resultCh)
	if res.cmdErr != "" {
		t.Fatalf("msg send --wait-read error = %q", res.cmdErr)
	}

	var out msgSendConfirmTestOutput
	if err := json.Unmarshal([]byte(res.output), &out); err != nil {
		t.Fatalf("unmarshal msg send --wait-read output: %v\n%s", err, res.output)
	}
	if out.Confirm.Status != "read" || !out.Confirm.Satisfied || len(out.Confirm.Deliveries) != 1 {
		t.Fatalf("read confirm = %#v, want satisfied single delivery", out.Confirm)
	}
	delivery := out.Confirm.Deliveries[0]
	if delivery.Recipient.Name != "pane-2" || delivery.ReadAt == "" || delivery.AckedAt != "" || delivery.AckStatus != "" {
		t.Fatalf("read delivery = %#v, want read without ack", delivery)
	}
}

func TestMsgCommandSendWaitAckTimeoutNamesPendingRecipients(t *testing.T) {
	t.Parallel()

	srv, sess, p1, p2, cleanup := newMailboxTestSession(t)
	defer cleanup()

	res := runTestCommand(t, srv, sess, "msg", "send",
		"--from", p1.Meta.Name,
		"--to", p2.Meta.Name,
		"--body", "body",
		"--wait-ack",
		"--timeout", "10ms")
	if res.cmdErr == "" {
		t.Fatalf("msg send --wait-ack timeout succeeded with output %q, want error", res.output)
	}
	if !strings.Contains(res.cmdErr, "timed out waiting for ack") || !strings.Contains(res.cmdErr, "pane-2") {
		t.Fatalf("timeout error = %q, want ack timeout naming pane-2", res.cmdErr)
	}
}

func TestMsgCommandSendTimeoutRequiresWaitCondition(t *testing.T) {
	t.Parallel()

	srv, sess, p1, p2, cleanup := newMailboxTestSession(t)
	defer cleanup()

	res := runTestCommand(t, srv, sess, "msg", "send",
		"--from", p1.Meta.Name,
		"--to", p2.Meta.Name,
		"--body", "body",
		"--timeout", "10ms")
	if res.cmdErr != "--timeout requires --wait-read or --wait-ack" {
		t.Fatalf("msg send --timeout error = %q, want wait requirement", res.cmdErr)
	}
}

func addMailboxTestPane(t *testing.T, sess *Session, id uint32, name string) *mux.Pane {
	t.Helper()

	pane := newTestPane(sess, id, name)
	mustSessionMutation(t, sess, func(sess *Session) {
		w := sess.activeWindow()
		if _, err := w.Split(mux.SplitHorizontal, pane); err != nil {
			t.Fatalf("Split: %v", err)
		}
		sess.Panes = w.Panes()
	})
	return pane
}

func waitForMailboxDelivery(t *testing.T, sess *Session, id mailbox.MessageID, recipientID uint32) {
	t.Helper()

	waitUntil(t, func() bool {
		return mustSessionQuery(t, sess, func(sess *Session) bool {
			_, err := sess.ensureMailbox().DeliverySummary(id, recipientID)
			return err == nil
		})
	})
}

func readMsgConfirmResult(t *testing.T, resultCh <-chan struct {
	output string
	cmdErr string
}) struct {
	output string
	cmdErr string
} {
	t.Helper()

	select {
	case res := <-resultCh:
		return res
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for msg send --wait result")
		return struct {
			output string
			cmdErr string
		}{}
	}
}
