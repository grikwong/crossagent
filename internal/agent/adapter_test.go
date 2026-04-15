package agent

import (
	"strings"
	"testing"
)

// TestAdapterRegistry_ContractsHoldForAllBuiltins enforces the basic
// Adapter contract on every registered adapter. This acts as a safety
// net: if someone adds a new adapter file and forgets Plan, or returns
// an empty Name, or forgets to register, this test fails — so we don't
// need per-adapter "is it wired up?" tests.
func TestAdapterRegistry_ContractsHoldForAllBuiltins(t *testing.T) {
	names := AdapterNames()
	if len(names) == 0 {
		t.Fatal("AdapterNames returned empty — no adapters registered")
	}

	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			ad, ok := AdapterFor(name)
			if !ok {
				t.Fatalf("AdapterFor(%q) returned !ok but name appears in AdapterNames", name)
			}
			if ad.Name() != name {
				t.Errorf("Name() = %q, want %q (registry key must match)", ad.Name(), name)
			}
			if ad.DisplayName() == "" {
				t.Error("DisplayName() is empty")
			}
			if ad.DefaultCommand() == "" {
				t.Error("DefaultCommand() is empty")
			}
			if !ValidAdapter(name) {
				t.Errorf("ValidAdapter(%q) = false, want true", name)
			}
			if !IsBuiltinAgent(name) {
				t.Errorf("IsBuiltinAgent(%q) = false, want true (every adapter backs a builtin)", name)
			}
		})
	}
}

// TestAdapterRegistry_BuiltinAgentDerivedFromAdapter pins the
// "one builtin agent per registered adapter" invariant. If we ever
// decide to break that, this test documents what needs to be updated.
func TestAdapterRegistry_BuiltinAgentDerivedFromAdapter(t *testing.T) {
	for _, name := range AdapterNames() {
		ad, _ := AdapterFor(name)
		ag, ok := builtinAgent(name)
		if !ok {
			t.Fatalf("builtinAgent(%q) returned !ok", name)
		}
		if ag.Name != ad.Name() || ag.Adapter != ad.Name() ||
			ag.Command != ad.DefaultCommand() || ag.DisplayName != ad.DisplayName() ||
			!ag.Builtin {
			t.Errorf("builtinAgent(%q) = %+v, not derived from adapter", name, ag)
		}
	}
}

// TestListAgents_BuiltinsFromRegistry confirms ListAgents returns one
// builtin per registered adapter in registration order.
func TestListAgents_BuiltinsFromRegistry(t *testing.T) {
	home := setupTestHome(t)
	_ = home

	agents, err := ListAgents()
	if err != nil {
		t.Fatalf("ListAgents: %v", err)
	}
	names := AdapterNames()
	if len(agents) != len(names) {
		t.Fatalf("ListAgents returned %d, want %d (one builtin per adapter)",
			len(agents), len(names))
	}
	for i, n := range names {
		if agents[i].Name != n {
			t.Errorf("agents[%d].Name = %q, want %q (registration order)", i, agents[i].Name, n)
		}
		if !agents[i].Builtin {
			t.Errorf("agents[%d].Builtin = false, want true", i)
		}
	}
}

// TestValidAdapter_ErrorMessageEnumeratesRegistered guards the
// "Agent adapter must be one of: …" error. The message is derived from
// AdapterNames so adding a new adapter updates it automatically. This
// test pins the format.
func TestValidAdapter_ErrorMessageFromRegistry(t *testing.T) {
	setupTestHome(t)
	err := AddAgent("bogus-adapter-test", "no-such-adapter", "cmd", "Test")
	if err == nil {
		t.Fatal("expected error for invalid adapter")
	}
	msg := err.Error()
	for _, name := range AdapterNames() {
		if !strings.Contains(msg, name) {
			t.Errorf("error message should enumerate %q: %q", name, msg)
		}
	}
}
