//go:build windows
// +build windows

package taskmaster

import (
	"testing"
	"time"
)

func TestRelease(t *testing.T) {
	var rt RunningTask
	rt.Release()
}

func TestRunRegisteredTask(t *testing.T) {
	taskService := setupTaskService(t)
	testTask := createTestTask(taskService)

	runningTask, err := testTask.Run("3")
	if err != nil {
		t.Fatal(err)
	}
	runningTask.Release()
}

func TestRefreshRunningTask(t *testing.T) {
	taskService := setupTaskService(t)
	testTask := createTestTask(taskService)

	runningTask, err := testTask.Run("3")
	if err != nil {
		t.Fatal(err)
	}
	err = runningTask.Refresh()
	if err != nil {
		t.Fatal(err)
	}

	runningTask.Release()
}

func TestStopRunningTask(t *testing.T) {
	taskService := setupTaskService(t)
	testTask := createTestTask(taskService)

	runningTask, err := testTask.Run("9001")
	if err != nil {
		t.Fatal(err)
	}

	err = runningTask.Stop()
	if err != nil {
		t.Fatal(err)
	}
}

func TestGetInstancesRegisteredTask(t *testing.T) {
	taskService := setupTaskService(t)
	testTask := createTestTask(taskService)

	runningTasks := make(RunningTaskCollection, 5, 5)
	var err error

	// create a few running tasks so that there will be multiple instances
	// of the registered task running
	for i := range runningTasks {
		runningTasks[i], err = testTask.Run("3")
		if err != nil {
			t.Fatal(err)
		}
		time.Sleep(100 * time.Millisecond)
	}

	instances, err := testTask.GetInstances()
	if err != nil {
		t.Fatal(err)
	}

	if len(instances) != 5 {
		t.Fatalf("should have 5 instances, got %d instead", len(instances))
	}

	runningTasks.Stop()
	instances.Release()
}

func TestStopRegisteredTask(t *testing.T) {
	taskService := setupTaskService(t)
	testTask := createTestTask(taskService)

	var err error
	for i := 0; i < 5; i++ {
		_, err = testTask.Run("3")
		if err != nil {
			t.Fatal(err)
		}
	}

	err = testTask.Stop()
	if err != nil {
		t.Fatalf("error stopping tasks: %v", err)
	}
}
