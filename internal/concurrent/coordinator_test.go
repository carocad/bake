package concurrent

import (
	"context"
	"math"
	"strconv"
	"strings"
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

func TestSerialCoordination(t *testing.T) {
	// arrange
	preData := []string{"1", "2", "3", "4", "5"}
	data := make([]fakeAddress, 0)
	for index, value := range preData {
		data = append(data, fakeAddress{value, preData[:index]})
	}

	coordinator := NewCoordinator[fakeAddress](context.TODO(), DefaultParallelism)
	result := make([][]string, len(data))
	start := time.Now()
	for index, v := range data {
		index, v := index, v
		coordinator.Do(v, data[:index], func() error {
			time.Sleep(200 * time.Millisecond)
			result[index] = preData[:index]
			return nil
		})
	}

	err := coordinator.Wait()
	end := time.Now()
	if err != nil {
		t.Fatal(err)
	}

	last := len(result) - 1
	if strings.Join(result[last], ",") != strings.Join(preData[:last], ",") {
		t.Errorf("expected an slice of %v but got %v", data[:last], result[last])
	}

	if strings.Join(result[0], ",") != strings.Join(nil, ",") {
		t.Errorf("expected an slice of %v but got %v", nil, result[0])
	}

	duration := end.Sub(start)
	if diff := math.Abs(duration.Seconds() - 1); diff > tolerance {
		t.Errorf("expected around 1 seconds but got %f", duration.Seconds())
	}
}

func TestParallelCoordination(t *testing.T) {
	preData := []string{"1", "2", "3", "4", "5"}
	data := make([]fakeAddress, 0)
	for _, value := range preData {
		data = append(data, fakeAddress{value, nil})
	}
	coordinator := NewCoordinator[fakeAddress](context.TODO(), DefaultParallelism)
	start := time.Now()
	for _, v := range data {
		coordinator.Do(v, nil, func() error {
			time.Sleep(200 * time.Millisecond)
			return nil
		})
	}

	err := coordinator.Wait()
	end := time.Now()
	if err != nil {
		t.Fatal(err)
	}

	duration := end.Sub(start)
	if diff := math.Abs(duration.Seconds() - 0.4); diff > tolerance {
		t.Errorf("expected around 0.4 seconds but got %f", duration.Seconds())
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

	coordinator := NewCoordinator[fakeAddress](context.TODO(), DefaultParallelism)
	counter := 0
	result := make([][]string, len(data))
	start := time.Now()
	for _, addr := range data {
		// duplicates the logic of topo.dependencies but if I import it, it creates a dependency cycle :(
		deps := make([]fakeAddress, 0)
		for _, v := range addr.deps {
			index, err := strconv.Atoi(v)
			if err != nil {
				t.Fatal(err)
			}

			deps = append(deps, data[index-1])
		}

		t.Log(addr.name, deps)

		id, index := addr, counter
		coordinator.Do(id, deps, func() error {
			t.Log(id.name + "-> doing")
			time.Sleep(200 * time.Millisecond)
			result[index] = data[index].deps
			t.Log(id.name + "-> done")
			return nil
		})
		counter++
	}

	err := coordinator.Wait()
	end := time.Now()
	if err != nil {
		t.Fatal(err)
	}

	last := len(result) - 1
	if strings.Join(result[last], ",") != strings.Join(data[4].deps, ",") {
		t.Errorf("expected an slice of %v but got %v", data[4], result[last])
	}

	if strings.Join(result[0], ",") != strings.Join(nil, ",") {
		t.Errorf("expected an slice of %v but got %v", nil, result[0])
	}

	duration := end.Sub(start)
	if diff := math.Abs(duration.Seconds() - 0.8); diff > tolerance {
		t.Errorf("expected around 1 seconds but got %f", duration.Seconds())
	}
}
