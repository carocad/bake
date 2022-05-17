package routine

import (
	"context"
	"math"
	"strings"
	"testing"
	"time"
)

const tolerance = 0.1

func TestSerialCoordination(t *testing.T) {
	data := []string{"1", "2", "3", "4", "5"}
	coordinator := WithContext(context.TODO())
	result := make([][]string, len(data))
	start := time.Now()
	for index, v := range data {
		index, v := index, v
		coordinator.Do(v, data[:index], func() error {
			time.Sleep(200 * time.Millisecond)
			result[index] = data[:index]
			return nil
		})
	}

	err := coordinator.Wait()
	end := time.Now()
	if err != nil {
		t.Fatal(err)
	}

	last := len(result) - 1
	if strings.Join(result[last], ",") != strings.Join(data[:last], ",") {
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
	data := []string{"1", "2", "3", "4", "5"}
	coordinator := WithContext(context.TODO())
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
	data := map[string][]string{
		"1": nil,        // duration = 0.2
		"2": {"1"},      // duration = 0.2 + 0.2 = 0.4
		"3": nil,        // duration = 0.2
		"4": {"3", "2"}, // duration = 0.2 + 0.4 = 0.6
		"5": {"4"},      // duration = 0.2 + 0.6 = 0.8
	}
	coordinator := WithContext(context.TODO())
	result := make([][]string, len(data))
	start := time.Now()
	index := 0
	for id, deps := range data {
		id, counter := id, index
		coordinator.Do(id, deps, func() error {
			time.Sleep(200 * time.Millisecond)
			result[counter] = data[id]
			return nil
		})
		index++
	}

	err := coordinator.Wait()
	end := time.Now()
	if err != nil {
		t.Fatal(err)
	}

	last := len(result) - 1
	if strings.Join(result[last], ",") != strings.Join(data["5"], ",") {
		t.Errorf("expected an slice of %v but got %v", data["5"], result[last])
	}

	if strings.Join(result[0], ",") != strings.Join(nil, ",") {
		t.Errorf("expected an slice of %v but got %v", nil, result[0])
	}

	duration := end.Sub(start)
	if diff := math.Abs(duration.Seconds() - 0.8); diff > tolerance {
		t.Errorf("expected around 1 seconds but got %f", duration.Seconds())
	}
}
