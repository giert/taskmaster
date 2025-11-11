//go:build windows
// +build windows

package taskmaster

import (
	"strings"
	"testing"
	"time"
)

func TestLocalConnect(t *testing.T) {
	setupTaskService(t)
}

func TestCreateTask(t *testing.T) {
	var err error
	taskService := setupTaskService(t)

	// test ExecAction
	execTaskDef := taskService.NewTaskDefinition()
	popCalc := ExecAction{
		Path: "calc.exe",
	}
	execTaskDef.AddAction(popCalc)

	_, _, err = taskService.CreateTask(testTaskPath("ExecAction"), execTaskDef, true)
	if err != nil {
		t.Fatal(err)
	}

	// test ComHandlerAction
	comHandlerDef := taskService.NewTaskDefinition()
	comHandlerDef.AddAction(ComHandlerAction{
		ClassID: "{F0001111-0000-0000-0000-0000FEEDACDC}",
	})

	_, _, err = taskService.CreateTask(testTaskPath("ComHandlerAction"), comHandlerDef, true)
	if err != nil {
		t.Fatal(err)
	}

	// test BootTrigger
	bootTriggerDef := taskService.NewTaskDefinition()
	bootTriggerDef.AddAction(popCalc)
	bootTriggerDef.AddTrigger(BootTrigger{})
	_, _, err = taskService.CreateTask(testTaskPath("BootTrigger"), bootTriggerDef, true)
	if err != nil {
		t.Fatal(err)
	}

	// test DailyTrigger
	dailyTriggerDef := taskService.NewTaskDefinition()
	dailyTriggerDef.AddAction(popCalc)
	dailyTriggerDef.AddTrigger(DailyTrigger{
		DayInterval: EveryDay,
		TaskTrigger: TaskTrigger{
			StartBoundary: time.Now(),
		},
	})
	_, _, err = taskService.CreateTask(testTaskPath("DailyTrigger"), dailyTriggerDef, true)
	if err != nil {
		t.Fatal(err)
	}

	// test EventTrigger
	eventTriggerDef := taskService.NewTaskDefinition()
	eventTriggerDef.AddAction(popCalc)
	subscription := "<QueryList> <Query Id='1'> <Select Path='System'>*[System/Level=2]</Select></Query></QueryList>"
	eventTriggerDef.AddTrigger(EventTrigger{
		Subscription: subscription,
	})
	_, _, err = taskService.CreateTask(testTaskPath("EventTrigger"), eventTriggerDef, true)
	if err != nil {
		t.Fatal(err)
	}

	// test IdleTrigger
	idleTriggerDef := taskService.NewTaskDefinition()
	idleTriggerDef.AddAction(popCalc)
	idleTriggerDef.AddTrigger(IdleTrigger{})
	_, _, err = taskService.CreateTask(testTaskPath("IdleTrigger"), idleTriggerDef, true)
	if err != nil {
		t.Fatal(err)
	}

	// test LogonTrigger
	logonTriggerDef := taskService.NewTaskDefinition()
	logonTriggerDef.AddAction(popCalc)
	logonTriggerDef.AddTrigger(LogonTrigger{})
	_, _, err = taskService.CreateTask(testTaskPath("LogonTrigger"), logonTriggerDef, true)
	if err != nil {
		t.Fatal(err)
	}

	// test MonthlyDOWTrigger
	monthlyDOWTriggerDef := taskService.NewTaskDefinition()
	monthlyDOWTriggerDef.AddAction(popCalc)
	monthlyDOWTriggerDef.AddTrigger(MonthlyDOWTrigger{
		DaysOfWeek:   Monday | Friday,
		WeeksOfMonth: First,
		MonthsOfYear: January | February,
		TaskTrigger: TaskTrigger{
			StartBoundary: time.Now(),
		},
	})
	_, _, err = taskService.CreateTask(testTaskPath("MonthlyDOWTrigger"), monthlyDOWTriggerDef, true)
	if err != nil {
		t.Fatal(err)
	}

	// test MonthlyTrigger
	monthlyTriggerDef := taskService.NewTaskDefinition()
	monthlyTriggerDef.AddAction(popCalc)
	monthlyTriggerDef.AddTrigger(MonthlyTrigger{
		DaysOfMonth:  3,
		MonthsOfYear: February | March,
		TaskTrigger: TaskTrigger{
			StartBoundary: time.Now(),
		},
	})
	_, _, err = taskService.CreateTask(testTaskPath("MonthlyTrigger"), monthlyTriggerDef, true)
	if err != nil {
		t.Fatal(err)
	}

	// test RegistrationTrigger
	registrationTriggerDef := taskService.NewTaskDefinition()
	registrationTriggerDef.AddAction(popCalc)
	registrationTriggerDef.AddTrigger(RegistrationTrigger{})
	_, _, err = taskService.CreateTask(testTaskPath("RegistrationTrigger"), registrationTriggerDef, true)
	if err != nil {
		t.Fatal(err)
	}

	// test SessionStateChangeTrigger
	sessionStateChangeTriggerDef := taskService.NewTaskDefinition()
	sessionStateChangeTriggerDef.AddAction(popCalc)
	sessionStateChangeTriggerDef.AddTrigger(SessionStateChangeTrigger{
		StateChange: TASK_SESSION_LOCK,
	})
	_, _, err = taskService.CreateTask(testTaskPath("SessionStateChangeTrigger"), sessionStateChangeTriggerDef, true)
	if err != nil {
		t.Fatal(err)
	}

	// test TimeTrigger
	timeTriggerDef := taskService.NewTaskDefinition()
	timeTriggerDef.AddAction(popCalc)
	timeTriggerDef.AddTrigger(TimeTrigger{
		TaskTrigger: TaskTrigger{
			StartBoundary: time.Now(),
		},
	})
	_, _, err = taskService.CreateTask(testTaskPath("TimeTrigger"), timeTriggerDef, true)
	if err != nil {
		t.Fatal(err)
	}

	// test WeeklyTrigger
	weeklyTriggerDef := taskService.NewTaskDefinition()
	weeklyTriggerDef.AddAction(popCalc)
	weeklyTriggerDef.AddTrigger(WeeklyTrigger{
		DaysOfWeek:   Tuesday | Thursday,
		WeekInterval: EveryOtherWeek,
		TaskTrigger: TaskTrigger{
			StartBoundary: time.Now(),
		},
	})
	_, _, err = taskService.CreateTask(testTaskPath("WeeklyTrigger"), weeklyTriggerDef, true)
	if err != nil {
		t.Fatal(err)
	}

	// test trying to create task where a task at the same path already exists and the 'overwrite' is set to false
	_, taskCreated, err := taskService.CreateTask(testTaskPath("TimeTrigger"), timeTriggerDef, false)
	if err != nil {
		t.Fatal(err)
	}
	if taskCreated {
		t.Fatal("task shouldn't have been created")
	}
}

func TestUpdateTask(t *testing.T) {
	taskService := setupTaskService(t)
	testTask := createTestTask(taskService)

	testTask.Definition.RegistrationInfo.Author = "Big Chungus"
	_, err := taskService.UpdateTask(testTaskPath("TestTask"), testTask.Definition)
	if err != nil {
		t.Fatal(err)
	}

	testTask, err = taskService.GetRegisteredTask(testTaskPath("TestTask"))
	if err != nil {
		t.Fatal(err)
	}
	if testTask.Definition.RegistrationInfo.Author != "Big Chungus" {
		t.Fatal("task was not updated")
	}
}

func TestGetRegisteredTasks(t *testing.T) {
	taskService := setupTaskService(t)
	createTestTask(taskService)

	rtc, err := taskService.GetRegisteredTasks()
	if err != nil {
		t.Fatal(err)
	}

	var found bool
	for _, task := range rtc {
		if task.Path == testTaskPath("TestTask") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected to find %s in registered tasks", testTaskPath("TestTask"))
	}
}

func TestGetTaskFolders(t *testing.T) {
	taskService := setupTaskService(t)

	tf, err := taskService.GetTaskFolders()
	if err != nil {
		t.Fatal(err)
	}
	tf.Release()
}

func TestDeleteTask(t *testing.T) {
	taskService := setupTaskService(t)
	createTestTask(taskService)

	err := taskService.DeleteTask(testTaskPath("TestTask"))
	if err != nil {
		t.Fatal(err)
	}

	deletedTask, err := taskService.GetRegisteredTask(testTaskPath("TestTask"))
	if err == nil {
		t.Fatal("task shouldn't still exist")
	}
	deletedTask.Release()
}

func TestDeleteFolder(t *testing.T) {
	taskService := setupTaskService(t)
	createTestTask(taskService)

	var folderDeleted bool
	folderDeleted, err := taskService.DeleteFolder(testTaskRoot, false)
	if err != nil {
		t.Fatal(err)
	}
	if folderDeleted == true {
		t.Error("folder shouldn't have been deleted")
	}

	folderDeleted, err = taskService.DeleteFolder(testTaskRoot, true)
	if err != nil {
		t.Fatal(err)
	}
	if folderDeleted == false {
		t.Error("folder should have been deleted")
	}

	tasks, err := taskService.GetRegisteredTasks()
	if err != nil {
		t.Fatal(err)
	}
	taskmasterFolder, err := taskService.GetTaskFolder(testTaskRoot)
	if err == nil {
		t.Fatal("folder shouldn't exist")
	}
	if taskmasterFolder.Name != "" {
		t.Error("folder struct should be defaultly constructed")
	}
	for _, task := range tasks {
		if strings.Split(task.Path, "\\")[1] == testTaskFolderName {
			t.Error("task should've been deleted")
		}
	}
}
