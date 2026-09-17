package ui

import (
	"testing"
	"time"
)

func TestTickCmdProducesTickMsg(t *testing.T) {
	if _, ok := tickCmd(time.Millisecond)().(tickMsg); !ok {
		t.Error("tickCmd must produce a tickMsg")
	}
}

func TestTickTriggersRefreshAndReschedules(t *testing.T) {
	m := testModel(t, 3)
	m = m.WithDeps(Deps{Provider: &fakeProvider{}})
	before := m.reqID

	next, cmd := m.Update(tickMsg(time.Now()))
	m = next.(Model)

	if m.reqID != before+1 {
		t.Errorf("reqID = %d, want %d — the tick must fetch", m.reqID, before+1)
	}
	if cmd == nil {
		t.Fatal("the tick must return commands, including the next tick")
	}
}

// If the reschedule is conditional, a suppressed refresh kills the
// ticker for the rest of the session.
func TestTickRescheduesEvenWhenRefreshIsSuppressed(t *testing.T) {
	cases := map[string]func(Model) Model{
		"no cities": func(m Model) Model {
			m.cities = nil
			m.cfg.Cities = nil
			return m
		},
		"backoff": func(m Model) Model {
			m.backoffTil = time.Now().Add(time.Hour)
			return m
		},
		"no provider": func(m Model) Model {
			return m.WithDeps(Deps{})
		},
	}

	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			m := mutate(testModel(t, 3).WithDeps(Deps{Provider: &fakeProvider{}}))

			_, cmd := m.Update(tickMsg(time.Now()))
			if cmd == nil {
				t.Fatal("the next tick must still be scheduled")
			}
			// The command is the tick itself (tea.Batch returns a lone
			// command unchanged), but invoking it would sleep a real
			// config interval — minutes — so its message type is proven
			// by TestTickCmdProducesTickMsg instead.
			_ = cmd
		})
	}
}

func TestInitStartsTheTicker(t *testing.T) {
	m := testModel(t, 2)
	m = m.WithDeps(Deps{Provider: &fakeProvider{}})

	if m.Init() == nil {
		t.Error("Init must return commands")
	}
}
