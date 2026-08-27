package server

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/weill-labs/amux/internal/config"
	"github.com/weill-labs/amux/internal/mux"
	commandpkg "github.com/weill-labs/amux/internal/server/commands"
)

const layoutUsage = "usage: amux layout [ultra [--page N] [--rows R] [--cols C] [--no-auto-promote] | off | page <next|prev|N> | toggle | status]"

type layoutArgs struct {
	mode        string // "ultra" | "off" | "page" | "toggle" | "status"
	page        int    // 1-based; 0 = unchanged
	pageStep    int    // +1 / -1 for next/prev
	rows, cols  int    // 0 = config default
	autoPromote *bool
}

func parseLayoutArgs(args []string) (layoutArgs, error) {
	la := layoutArgs{mode: "status"}
	if len(args) == 0 {
		return la, nil
	}
	switch strings.ToLower(args[0]) {
	case "ultra", "ultrade":
		la.mode = "ultra"
	case "off", "default", "normal":
		la.mode = "off"
		return la, nil
	case "toggle":
		la.mode = "toggle"
	case "status":
		return la, nil
	case "page":
		la.mode = "page"
		if len(args) < 2 {
			return la, fmt.Errorf("%s", layoutUsage)
		}
		switch strings.ToLower(args[1]) {
		case "next", "n":
			la.pageStep = 1
		case "prev", "previous", "p":
			la.pageStep = -1
		default:
			n, err := strconv.Atoi(args[1])
			if err != nil || n < 1 {
				return la, fmt.Errorf("layout page: expected next, prev, or a page number >= 1")
			}
			la.page = n
		}
		return la, nil
	default:
		return la, fmt.Errorf("%s", layoutUsage)
	}
	rest := args[1:]
	for i := 0; i < len(rest); i++ {
		arg := rest[i]
		intFlag := func(name string) (int, error) {
			if i+1 >= len(rest) {
				return 0, fmt.Errorf("layout: %s requires a value", name)
			}
			i++
			n, err := strconv.Atoi(rest[i])
			if err != nil || n < 1 {
				return 0, fmt.Errorf("layout: %s expects a positive integer", name)
			}
			return n, nil
		}
		var err error
		switch arg {
		case "--page":
			la.page, err = intFlag(arg)
		case "--rows":
			la.rows, err = intFlag(arg)
		case "--cols":
			la.cols, err = intFlag(arg)
		case "--no-auto-promote":
			v := false
			la.autoPromote = &v
		case "--auto-promote":
			v := true
			la.autoPromote = &v
		default:
			return la, fmt.Errorf("%s", layoutUsage)
		}
		if err != nil {
			return la, err
		}
	}
	return la, nil
}

func loadUltraDefaults() (rows, cols int, autoPromote bool) {
	rows, cols, autoPromote = mux.DefaultUltraRows, mux.DefaultUltraCols, true
	cfg, err := config.Load(config.DefaultPath())
	if err != nil || cfg == nil {
		return
	}
	r, c, a := cfg.EffectiveUltra()
	return r, c, a
}

func ultraStatus(w *mux.Window) string {
	if w == nil || w.Ultra == nil {
		return "layout: default\n"
	}
	u := w.Ultra
	filled := 0
	for _, p := range u.Slots {
		if p != nil {
			filled++
		}
	}
	lead := "none"
	if w.LeadPaneID != 0 {
		lead = strconv.Itoa(int(w.LeadPaneID))
	}
	return fmt.Sprintf("layout: ultra %dx%d page %d/%d lead=%s slots=%d/%d auto_promote=%t\n",
		u.Cols, u.Rows, u.Page+1, u.PageCount(), lead, filled, len(u.Slots), u.AutoPromote)
}

func runLayout(ctx *CommandContext, actorPaneID uint32, args []string) commandpkg.Result {
	la, err := parseLayoutArgs(args)
	if err != nil {
		return commandpkg.Result{Err: err}
	}
	defRows, defCols, defAuto := loadUltraDefaults()
	return toCommandResult(ctx.Sess.enqueueCommandMutationContext(ctx.context(), func(mctx *MutationContext) commandMutationResult {
		w := mctx.windowForActor(actorPaneID)
		if w == nil {
			return commandMutationResult{err: fmt.Errorf("no session")}
		}
		switch la.mode {
		case "status":
			return commandMutationResult{output: ultraStatus(w)}
		case "off":
			if w.Ultra == nil {
				return commandMutationResult{output: "layout: default (already)\n"}
			}
			if err := w.DisableUltra(); err != nil {
				return commandMutationResult{err: err}
			}
			return commandMutationResult{output: "layout: default\n", broadcastLayout: true}
		case "toggle":
			if w.Ultra != nil {
				if err := w.DisableUltra(); err != nil {
					return commandMutationResult{err: err}
				}
				return commandMutationResult{output: "layout: default\n", broadcastLayout: true}
			}
			la.mode = "ultra"
			fallthrough
		case "ultra":
			rows, cols, auto := defRows, defCols, defAuto
			if w.Ultra != nil {
				rows, cols, auto = w.Ultra.Rows, w.Ultra.Cols, w.Ultra.AutoPromote
			}
			if la.rows > 0 {
				rows = la.rows
			}
			if la.cols > 0 {
				cols = la.cols
			}
			if la.autoPromote != nil {
				auto = *la.autoPromote
			}
			if err := w.EnableUltra(rows, cols, auto); err != nil {
				return commandMutationResult{err: err}
			}
			if la.page > 0 {
				if err := w.UltraSetPage(la.page - 1); err != nil {
					return commandMutationResult{err: err}
				}
			}
			return commandMutationResult{output: ultraStatus(w), broadcastLayout: true}
		case "page":
			if w.Ultra == nil {
				return commandMutationResult{err: fmt.Errorf("window is not in ultra layout (run `amux layout ultra`)")}
			}
			var err error
			if la.pageStep != 0 {
				err = w.UltraStepPage(la.pageStep)
			} else {
				err = w.UltraSetPage(la.page - 1)
			}
			if err != nil {
				return commandMutationResult{err: err}
			}
			return commandMutationResult{output: ultraStatus(w), broadcastLayout: true}
		}
		return commandMutationResult{err: fmt.Errorf("%s", layoutUsage)}
	}))
}

func cmdLayout(ctx *CommandContext) {
	ctx.applyCommandResult(runLayout(ctx, ctx.ActorPaneID, ctx.Args))
}

// ultraPromoteIdle bumps a pane that just went idle on a hidden ultra page
// onto the visible page (auto-promotion). Runs on the session event loop.
func (s *Session) ultraPromoteIdle(paneID uint32) bool {
	w := s.findWindowByPaneID(paneID)
	if w == nil || w.Ultra == nil || !w.Ultra.AutoPromote {
		return false
	}
	tracker := s.ensureIdleTracker()
	return w.UltraPromote(paneID, func(id uint32) bool { return !tracker.IsIdle(id) })
}
