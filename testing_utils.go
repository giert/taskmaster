//go:build windows
// +build windows

package taskmaster

import (
	"strings"
	"testing"
)

const (
	testTaskFolderName = "TaskmasterTests"
	testTaskRoot       = `\` + testTaskFolderName
)

func setupTaskService(t *testing.T) *TaskService {
	t.Helper()

	taskService, err := Connect()
	if err != nil {
		t.Fatalf("failed to connect to Task Scheduler: %v", err)
	}

	resetTestFolder(t, &taskService)

	t.Cleanup(func() {
		resetTestFolder(t, &taskService)
		taskService.Disconnect()
	})

	return &taskService
}

func resetTestFolder(t *testing.T, taskService *TaskService) {
	t.Helper()

	if taskService.taskFolderExist(testTaskRoot) {
		if _, err := taskService.DeleteFolder(testTaskRoot, true); err != nil {
			t.Fatalf("failed to delete %s: %v", testTaskRoot, err)
		}
	}
}

func testTaskPath(parts ...string) string {
	if len(parts) == 0 {
		return testTaskRoot
	}

	cleaned := make([]string, 0, len(parts))
	for _, part := range parts {
		cleaned = append(cleaned, strings.Trim(part, "\\"))
	}

	return testTaskRoot + `\` + strings.Join(cleaned, `\`)
}

func createTestTask(taskSvc *TaskService) RegisteredTask {
	newTaskDef := taskSvc.NewTaskDefinition()
	newTaskDef.AddAction(ExecAction{
		Path: "cmd.exe",
		Args: "/c timeout $(Arg0)",
	})
	newTaskDef.Settings.MultipleInstances = TASK_INSTANCES_PARALLEL

	task, _, err := taskSvc.CreateTask(testTaskPath("TestTask"), newTaskDef, true)
	if err != nil {
		panic(err)
	}

	return task
}
