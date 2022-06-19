package module

import (
	"bake/internal/functional"
	"bake/internal/lang"
	"bake/internal/lang/config"
	"context"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
)

const tolerance = 0.1

type fakeAddress struct {
	name string
	deps []string
}

func (s fakeAddress) GetName() string {
	return string(s.name)
}

func (s fakeAddress) GetPath() cty.Path {
	return cty.GetAttrPath(s.GetName())
}

func (s fakeAddress) GetFilename() string {
	return s.GetName()
}

func (s fakeAddress) Dependencies() ([]hcl.Traversal, hcl.Diagnostics) {
	result := make([]hcl.Traversal, 0)
	for _, v := range s.deps {
		result = append(result, []hcl.Traverser{hcl.TraverseRoot{Name: v}})
	}

	return result, nil
}

func (s fakeAddress) Decode(ctx *hcl.EvalContext) ([]lang.Action, hcl.Diagnostics) {
	return []lang.Action{s}, nil
}

func (s fakeAddress) CTY() cty.Value {
	return cty.StringVal(s.name)
}

func (s fakeAddress) Hash() *config.Hash {
	return nil
}

func (s fakeAddress) Plan(state config.State) (bool, string, hcl.Diagnostics) {
	return true, fmt.Sprintf(`refreshing "%s"`, lang.AddressToString(s)), nil
}

func (s fakeAddress) Apply(ctx context.Context, state *config.State) hcl.Diagnostics {
	time.Sleep(200 * time.Millisecond)
	return nil
}

func TestSerialCoordination(t *testing.T) {
	// arrange
	preData := []string{"1", "2", "3", "4", "5"}
	data := make([]lang.RawAddress, 0)
	for index, value := range preData {
		data = append(data, fakeAddress{value, preData[:index]})
	}

	state, err := config.NewState()
	if err != nil {
		t.Fatal(err)
	}

	coordinator := NewCoordinator(context.TODO(), int(state.Parallelism))
	start := time.Now()
	actions, diags := coordinator.Do(state, data[len(data)-1], data)
	if diags.HasErrors() {
		t.Fatal(diags)
	}

	end := time.Now()
	last := len(actions) - 1
	if lang.AddressToString(actions[last]) != preData[last] {
		t.Errorf("expected an slice of %v but got %v", data[:last], actions[last])
	}

	if lang.AddressToString(actions[0]) != preData[0] {
		t.Errorf("expected an slice of %v but got %v", nil, actions[0])
	}

	duration := end.Sub(start)
	if diff := math.Abs(duration.Seconds() - 1); diff > tolerance {
		t.Errorf("expected around 1 seconds but got %f", duration.Seconds())
	}
}

func TestParallelCoordination(t *testing.T) {
	preData := []string{"1", "2", "3", "4", "5"}
	data := make([]lang.RawAddress, 0)
	for _, value := range preData {
		data = append(data, fakeAddress{value, nil})
	}

	state, err := config.NewState()
	if err != nil {
		t.Fatal(err)
	}

	coordinator := NewCoordinator(context.TODO(), int(state.Parallelism))
	start := time.Now()
	_, diags := coordinator.Do(state, data[len(data)-1], data)
	if diags.HasErrors() {
		t.Fatal(diags)
	}

	end := time.Now()
	duration := end.Sub(start)
	if diff := math.Abs(duration.Seconds() - 0.2); diff > tolerance {
		t.Errorf("expected around 0.2 seconds but got %f", duration.Seconds())
	}
}

func TestCustomCoordination(t *testing.T) {
	// duration = self + max(deps)
	data := []fakeAddress{{
		"1", nil, // duration = 0.2
	}, {
		"2", []string{"1"}, // duration = 0.2 + 0.2 = 0.4
	}, {
		"3", nil, // duration = 0.2
	}, {
		"4", []string{"3", "2"}, // duration = 0.2 + 0.4 = 0.6
	}, {
		"5", []string{"4"}, // duration = 0.2 + 0.6 = 0.8
	}}

	addresses := functional.Map(data, func(f fakeAddress) lang.RawAddress { return f })
	state, err := config.NewState()
	if err != nil {
		t.Fatal(err)
	}

	coordinator := NewCoordinator(context.TODO(), int(state.Parallelism))
	start := time.Now()
	actions, diags := coordinator.Do(state, data[len(data)-1], addresses)
	if diags.HasErrors() {
		t.Fatal(diags)
	}

	end := time.Now()
	last := len(actions) - 1
	if lang.AddressToString(actions[last]) != data[4].name {
		t.Errorf("expected last action %s but got %s", data[4].GetName(), lang.AddressToString(actions[last]))
	}

	if lang.AddressToString(actions[0]) != data[0].name && lang.AddressToString(actions[0]) != data[2].name {
		t.Errorf("expected first action %s but got %s", data[0].name, lang.AddressToString(actions[0]))
	}

	duration := end.Sub(start)
	if diff := math.Abs(duration.Seconds() - 0.8); diff > tolerance {
		t.Errorf("expected around 0.8 seconds but got %f", duration.Seconds())
	}
}
