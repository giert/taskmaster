//go:build windows
// +build windows

package taskmaster

import (
	"errors"
	"fmt"
	"math"
	"time"

	ole "github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
)

func parseRunningTask(task *ole.IDispatch) (RunningTask, error) {
	var err error

	currentAction, err := oleutil.GetProperty(task, "CurrentAction")
	if err != nil {
		return RunningTask{}, getRunningTaskError(err)
	}
	enginePID, err := oleutil.GetProperty(task, "EnginePid")
	if err != nil {
		return RunningTask{}, getRunningTaskError(err)
	}
	instanceGUID, err := oleutil.GetProperty(task, "InstanceGuid")
	if err != nil {
		return RunningTask{}, getRunningTaskError(err)
	}
	name, err := oleutil.GetProperty(task, "Name")
	if err != nil {
		return RunningTask{}, getRunningTaskError(err)
	}
	path, err := oleutil.GetProperty(task, "Path")
	if err != nil {
		return RunningTask{}, getRunningTaskError(err)
	}
	state, err := oleutil.GetProperty(task, "State")
	if err != nil {
		return RunningTask{}, getRunningTaskError(err)
	}

	runningTask := RunningTask{
		taskObj:       task,
		CurrentAction: currentAction.ToString(),
		EnginePID:     uint(enginePID.Val),
		InstanceGUID:  instanceGUID.ToString(),
		Name:          name.ToString(),
		Path:          path.ToString(),
		State:         TaskState(state.Val),
	}

	return runningTask, nil
}

func parseRegisteredTask(task *ole.IDispatch) (RegisteredTask, string, error) {
	h := &oleHelper{}

	name := h.getString(task, "Name")
	path := h.getString(task, "Path")
	enabled := h.getBool(task, "Enabled")
	state := TaskState(h.getInt(task, "State"))
	missedRuns := uint(h.getInt(task, "NumberOfMissedRuns"))
	nextRunTime := variantTimeOrZero(h.getVariant(task, "NextRunTime"))
	lastRunTime := variantTimeOrZero(h.getVariant(task, "LastRunTime"))
	lastTaskResult := TaskResult(h.getInt(task, "LastTaskResult"))
	if h.err != nil {
		return RegisteredTask{}, path, h.err
	}

	definition := h.getObject(task, "Definition")
	if h.err != nil {
		return RegisteredTask{}, path, h.err
	}
	defer definition.Release()

	actions := h.getObject(definition, "Actions")
	if h.err != nil {
		return RegisteredTask{}, path, h.err
	}
	defer actions.Release()

	context := h.getString(actions, "Context")
	if h.err != nil {
		return RegisteredTask{}, path, h.err
	}

	var taskActions []Action
	err := oleutil.ForEach(actions, func(v *ole.VARIANT) error {
		action := v.ToIDispatch()
		defer action.Release()

		taskAction, err := parseTaskAction(action)
		if err != nil {
			return err
		}

		taskActions = append(taskActions, taskAction)

		return nil
	})
	if err != nil {
		return RegisteredTask{}, path, fmt.Errorf("error parsing IAction object: %w", err)
	}

	principalObj := h.getObject(definition, "Principal")
	if h.err != nil {
		return RegisteredTask{}, path, h.err
	}
	defer principalObj.Release()
	taskPrincipal, err := parsePrincipal(principalObj)
	if err != nil {
		return RegisteredTask{}, path, fmt.Errorf("error parsing IPrincipal object: %w", err)
	}

	xmlText := h.getString(definition, "XmlText")
	if h.err != nil {
		return RegisteredTask{}, path, h.err
	}

	regInfo := h.getObject(definition, "RegistrationInfo")
	if h.err != nil {
		return RegisteredTask{}, path, h.err
	}
	defer regInfo.Release()
	registrationInfo, err := parseRegistrationInfo(regInfo)
	if err != nil {
		return RegisteredTask{}, path, fmt.Errorf("error parsing IRegistrationInfo object: %w", err)
	}

	settings := h.getObject(definition, "Settings")
	if h.err != nil {
		return RegisteredTask{}, path, h.err
	}
	defer settings.Release()
	taskSettings, err := parseTaskSettings(settings)
	if err != nil {
		return RegisteredTask{}, path, fmt.Errorf("error parsing ITaskSettings object: %w", err)
	}

	triggers := h.getObject(definition, "Triggers")
	if h.err != nil {
		return RegisteredTask{}, path, h.err
	}
	defer triggers.Release()

	var taskTriggers []Trigger
	err = oleutil.ForEach(triggers, func(v *ole.VARIANT) error {
		trigger := v.ToIDispatch()
		defer trigger.Release()

		taskTrigger, err := parseTaskTrigger(trigger)
		if err != nil {
			return err
		}
		taskTriggers = append(taskTriggers, taskTrigger)

		return nil
	})
	if err != nil {
		return RegisteredTask{}, path, fmt.Errorf("error parsing ITrigger object: %w", err)
	}

	taskDef := Definition{
		Actions:          taskActions,
		Context:          context,
		Principal:        taskPrincipal,
		Settings:         *taskSettings,
		RegistrationInfo: *registrationInfo,
		Triggers:         taskTriggers,
		XMLText:          xmlText,
	}

	registeredTask := RegisteredTask{
		taskObj:        task,
		Name:           name,
		Path:           path,
		Definition:     taskDef,
		Enabled:        enabled,
		State:          state,
		MissedRuns:     missedRuns,
		NextRunTime:    nextRunTime,
		LastRunTime:    lastRunTime,
		LastTaskResult: lastTaskResult,
	}

	return registeredTask, path, nil
}

func parseTaskAction(action *ole.IDispatch) (Action, error) {
	h := &oleHelper{}

	id := h.getString(action, "Id")
	actionType := TaskActionType(h.getInt(action, "Type"))
	if h.err != nil {
		return nil, h.err
	}

	switch actionType {
	case TASK_ACTION_EXEC:
		args := h.getString(action, "Arguments")
		path := h.getString(action, "Path")
		workingDir := h.getString(action, "WorkingDirectory")
		if h.err != nil {
			return nil, h.err
		}

		execAction := ExecAction{
			ID:         id,
			Path:       path,
			Args:       args,
			WorkingDir: workingDir,
		}

		return execAction, nil
	case TASK_ACTION_COM_HANDLER:
		classID := h.getString(action, "ClassId")
		data := h.getString(action, "Data")
		if h.err != nil {
			return nil, h.err
		}

		comHandlerAction := ComHandlerAction{
			ID:      id,
			ClassID: classID,
			Data:    data,
		}

		return comHandlerAction, nil
	default:
		return nil, errors.New("unsupported IAction type")
	}
}

func parsePrincipal(principleObj *ole.IDispatch) (Principal, error) {
	h := &oleHelper{}

	name := h.getString(principleObj, "DisplayName")
	groupID := h.getString(principleObj, "GroupId")
	id := h.getString(principleObj, "Id")
	logonType := TaskLogonType(h.getInt(principleObj, "LogonType"))
	runLevel := TaskRunLevel(h.getInt(principleObj, "RunLevel"))
	userID := h.getString(principleObj, "UserId")
	if h.err != nil {
		return Principal{}, h.err
	}

	principle := Principal{
		Name:      name,
		GroupID:   groupID,
		ID:        id,
		LogonType: logonType,
		RunLevel:  runLevel,
		UserID:    userID,
	}

	return principle, nil
}

func parseRegistrationInfo(regInfo *ole.IDispatch) (*RegistrationInfo, error) {
	h := &oleHelper{}

	author := h.getString(regInfo, "Author")
	dateStr := h.getString(regInfo, "Date")
	description := h.getString(regInfo, "Description")
	documentation := h.getString(regInfo, "Documentation")
	securityDescriptor := h.getString(regInfo, "SecurityDescriptor")
	source := h.getString(regInfo, "Source")
	uri := h.getString(regInfo, "URI")
	version := h.getString(regInfo, "Version")
	if h.err != nil {
		return nil, h.err
	}

	date, err := TaskDateToTime(dateStr)
	if err != nil {
		return nil, fmt.Errorf("error parsing Date field: %w", err)
	}

	registrationInfo := &RegistrationInfo{
		Author:             author,
		Date:               date,
		Description:        description,
		Documentation:      documentation,
		SecurityDescriptor: securityDescriptor,
		Source:             source,
		URI:                uri,
		Version:            version,
	}

	return registrationInfo, nil
}

func parseTaskSettings(settings *ole.IDispatch) (*TaskSettings, error) {
	h := &oleHelper{}

	allowDemandStart := h.getBool(settings, "AllowDemandStart")
	allowHardTerminate := h.getBool(settings, "AllowHardTerminate")
	compatibility := TaskCompatibility(h.getInt(settings, "Compatibility"))
	deleteExpiredTaskAfter := h.getString(settings, "DeleteExpiredTaskAfter")
	dontStartOnBatteries := h.getBool(settings, "DisallowStartIfOnBatteries")
	enabled := h.getBool(settings, "Enabled")
	timeLimitStr := h.getString(settings, "ExecutionTimeLimit")
	hidden := h.getBool(settings, "Hidden")
	if h.err != nil {
		return nil, h.err
	}
	timeLimit, err := StringToPeriod(timeLimitStr)
	if err != nil {
		return nil, fmt.Errorf("error parsing ExecutionTimeLimit field: %w", err)
	}

	idleSettings := h.getObject(settings, "IdleSettings")
	if h.err != nil {
		return nil, h.err
	}
	defer idleSettings.Release()
	idleDurationStr := h.getString(idleSettings, "IdleDuration")
	restartOnIdle := h.getBool(idleSettings, "RestartOnIdle")
	stopOnIdleEnd := h.getBool(idleSettings, "StopOnIdleEnd")
	waitTimeoutStr := h.getString(idleSettings, "WaitTimeout")
	if h.err != nil {
		return nil, h.err
	}
	idleDuration, err := StringToPeriod(idleDurationStr)
	if err != nil {
		return nil, fmt.Errorf("error parsing IdleDuration field: %w", err)
	}
	waitTimeOut, err := StringToPeriod(waitTimeoutStr)
	if err != nil {
		return nil, fmt.Errorf("error parsing WaitTimeout field: %w", err)
	}

	multipleInstances := TaskInstancesPolicy(h.getInt(settings, "MultipleInstances"))

	networkSettings := h.getObject(settings, "NetworkSettings")
	if h.err != nil {
		return nil, h.err
	}
	defer networkSettings.Release()
	id := h.getString(networkSettings, "Id")
	networkName := h.getString(networkSettings, "Name")

	priority := uint(h.getInt(settings, "Priority"))
	restartCount := uint(h.getInt(settings, "RestartCount"))
	restartIntervalStr := h.getString(settings, "RestartInterval")
	runOnlyIfIdle := h.getBool(settings, "RunOnlyIfIdle")
	runOnlyIfNetworkAvailable := h.getBool(settings, "RunOnlyIfNetworkAvailable")
	startWhenAvailable := h.getBool(settings, "StartWhenAvailable")
	stopIfGoingOnBatteries := h.getBool(settings, "StopIfGoingOnBatteries")
	wakeToRun := h.getBool(settings, "WakeToRun")
	if h.err != nil {
		return nil, h.err
	}
	restartInterval, err := StringToPeriod(restartIntervalStr)
	if err != nil {
		return nil, fmt.Errorf("error parsing RestartInterval field: %w", err)
	}

	idleTaskSettings := IdleSettings{
		IdleDuration:  idleDuration,
		RestartOnIdle: restartOnIdle,
		StopOnIdleEnd: stopOnIdleEnd,
		WaitTimeout:   waitTimeOut,
	}

	networkTaskSettings := NetworkSettings{
		ID:   id,
		Name: networkName,
	}

	taskSettings := &TaskSettings{
		AllowDemandStart:          allowDemandStart,
		AllowHardTerminate:        allowHardTerminate,
		Compatibility:             compatibility,
		DeleteExpiredTaskAfter:    deleteExpiredTaskAfter,
		DontStartOnBatteries:      dontStartOnBatteries,
		Enabled:                   enabled,
		TimeLimit:                 timeLimit,
		Hidden:                    hidden,
		IdleSettings:              idleTaskSettings,
		MultipleInstances:         multipleInstances,
		NetworkSettings:           networkTaskSettings,
		Priority:                  priority,
		RestartCount:              restartCount,
		RestartInterval:           restartInterval,
		RunOnlyIfIdle:             runOnlyIfIdle,
		RunOnlyIfNetworkAvailable: runOnlyIfNetworkAvailable,
		StartWhenAvailable:        startWhenAvailable,
		StopIfGoingOnBatteries:    stopIfGoingOnBatteries,
		WakeToRun:                 wakeToRun,
	}

	return taskSettings, nil
}

func parseTaskTrigger(trigger *ole.IDispatch) (Trigger, error) {
	h := &oleHelper{}

	enabled := h.getBool(trigger, "Enabled")
	endBoundaryStr := h.getString(trigger, "EndBoundary")
	executionTimeLimitStr := h.getString(trigger, "ExecutionTimeLimit")
	id := h.getString(trigger, "Id")
	if h.err != nil {
		return nil, h.err
	}
	endBoundary, err := TaskDateToTime(endBoundaryStr)
	if err != nil {
		return nil, fmt.Errorf("error parsing EndBoundary field: %w", err)
	}
	executionTimeLimit, err := StringToPeriod(executionTimeLimitStr)
	if err != nil {
		return nil, fmt.Errorf("error parsing ExecutionTimeLimit field: %w", err)
	}

	repetition := h.getObject(trigger, "Repetition")
	if h.err != nil {
		return nil, h.err
	}
	defer repetition.Release()
	durationStr := h.getString(repetition, "Duration")
	intervalStr := h.getString(repetition, "Interval")
	stopAtDurationEnd := h.getBool(repetition, "StopAtDurationEnd")
	if h.err != nil {
		return nil, h.err
	}
	duration, err := StringToPeriod(durationStr)
	if err != nil {
		return nil, fmt.Errorf("error parsing Duration field: %w", err)
	}
	interval, err := StringToPeriod(intervalStr)
	if err != nil {
		return nil, fmt.Errorf("error parsing Interval field: %w", err)
	}

	startBoundaryStr := h.getString(trigger, "StartBoundary")
	triggerType := TaskTriggerType(h.getInt(trigger, "Type"))
	if h.err != nil {
		return nil, h.err
	}
	startBoundary, err := TaskDateToTime(startBoundaryStr)
	if err != nil {
		return nil, fmt.Errorf("error parsing StartBoundary field: %w", err)
	}

	taskTriggerObj := TaskTrigger{
		Enabled:            enabled,
		EndBoundary:        endBoundary,
		ExecutionTimeLimit: executionTimeLimit,
		ID:                 id,
		RepetitionPattern: RepetitionPattern{
			RepetitionDuration: duration,
			RepetitionInterval: interval,
			StopAtDurationEnd:  stopAtDurationEnd,
		},
		StartBoundary: startBoundary,
	}

	switch triggerType {
	case TASK_TRIGGER_BOOT:
		delayStr := h.getString(trigger, "Delay")
		if h.err != nil {
			return nil, h.err
		}
		delay, err := StringToPeriod(delayStr)
		if err != nil {
			return nil, fmt.Errorf("error parsing IBootTrigger object: error parsing Delay field: %w", err)
		}

		bootTrigger := BootTrigger{
			TaskTrigger: taskTriggerObj,
			Delay:       delay,
		}

		return bootTrigger, nil
	case TASK_TRIGGER_DAILY:
		daysInterval := DayInterval(h.getInt(trigger, "DaysInterval"))
		randomDelayStr := h.getString(trigger, "RandomDelay")
		if h.err != nil {
			return nil, h.err
		}
		randomDelay, err := StringToPeriod(randomDelayStr)
		if err != nil {
			return nil, fmt.Errorf("error parsing IDailyTrigger object: error parsing RandomDelay field: %w", err)
		}

		dailyTrigger := DailyTrigger{
			TaskTrigger: taskTriggerObj,
			DayInterval: daysInterval,
			RandomDelay: randomDelay,
		}

		return dailyTrigger, nil
	case TASK_TRIGGER_EVENT:
		delayStr := h.getString(trigger, "Delay")
		subscription := h.getString(trigger, "Subscription")
		if h.err != nil {
			return nil, h.err
		}
		delay, err := StringToPeriod(delayStr)
		if err != nil {
			return nil, fmt.Errorf("error parsing IEventTrigger object: error parsing Delay field: %w", err)
		}

		valueQueriesObj := h.getObject(trigger, "ValueQueries")
		if h.err != nil {
			return nil, h.err
		}
		defer valueQueriesObj.Release()

		valQueryMap := make(map[string]string)
		err = oleutil.ForEach(valueQueriesObj, func(v *ole.VARIANT) error {
			valueQuery := v.ToIDispatch()
			defer valueQuery.Release()

			vh := &oleHelper{}
			name := vh.getString(valueQuery, "Name")
			value := vh.getString(valueQuery, "Value")
			if vh.err != nil {
				return vh.err
			}

			valQueryMap[name] = value

			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("error parsing IEventTrigger ValueQueries: %w", err)
		}

		eventTrigger := EventTrigger{
			TaskTrigger:  taskTriggerObj,
			Delay:        delay,
			Subscription: subscription,
			ValueQueries: valQueryMap,
		}

		return eventTrigger, nil
	case TASK_TRIGGER_IDLE:
		idleTrigger := IdleTrigger{
			TaskTrigger: taskTriggerObj,
		}

		return idleTrigger, nil
	case TASK_TRIGGER_LOGON:
		delayStr := h.getString(trigger, "Delay")
		userID := h.getString(trigger, "UserId")
		if h.err != nil {
			return nil, h.err
		}
		delay, err := StringToPeriod(delayStr)
		if err != nil {
			return nil, fmt.Errorf("error parsing ILogonTrigger object: error parsing Delay field: %w", err)
		}

		logonTrigger := LogonTrigger{
			TaskTrigger: taskTriggerObj,
			Delay:       delay,
			UserID:      userID,
		}

		return logonTrigger, nil
	case TASK_TRIGGER_MONTHLYDOW:
		daysOfWeek := DayOfWeek(h.getInt(trigger, "DaysOfWeek"))
		monthsOfYear := Month(h.getInt(trigger, "MonthsOfYear"))
		randomDelayStr := h.getString(trigger, "RandomDelay")
		runOnLastWeekOfMonth := h.getBool(trigger, "RunOnLastWeekOfMonth")
		weeksOfMonth := Week(h.getInt(trigger, "WeeksOfMonth"))
		if h.err != nil {
			return nil, h.err
		}
		randomDelay, err := StringToPeriod(randomDelayStr)
		if err != nil {
			return nil, fmt.Errorf("error parsing IMonthlyDOWTrigger object: error parsing RandomDelay field: %w", err)
		}

		monthlyDOWTrigger := MonthlyDOWTrigger{
			TaskTrigger:          taskTriggerObj,
			DaysOfWeek:           daysOfWeek,
			MonthsOfYear:         monthsOfYear,
			RandomDelay:          randomDelay,
			RunOnLastWeekOfMonth: runOnLastWeekOfMonth,
			WeeksOfMonth:         weeksOfMonth,
		}

		return monthlyDOWTrigger, nil
	case TASK_TRIGGER_MONTHLY:
		daysOfMonth := DayOfMonth(h.getInt(trigger, "DaysOfMonth"))
		monthsOfYear := Month(h.getInt(trigger, "MonthsOfYear"))
		randomDelayStr := h.getString(trigger, "RandomDelay")
		runOnLastDayOfMonth := h.getBool(trigger, "RunOnLastDayOfMonth")
		if h.err != nil {
			return nil, h.err
		}
		randomDelay, err := StringToPeriod(randomDelayStr)
		if err != nil {
			return nil, fmt.Errorf("error parsing IMonthlyTrigger object: error parsing RandomDelay field: %w", err)
		}

		monthlyTrigger := MonthlyTrigger{
			TaskTrigger:         taskTriggerObj,
			DaysOfMonth:         daysOfMonth,
			MonthsOfYear:        monthsOfYear,
			RandomDelay:         randomDelay,
			RunOnLastDayOfMonth: runOnLastDayOfMonth,
		}

		return monthlyTrigger, nil
	case TASK_TRIGGER_REGISTRATION:
		delayStr := h.getString(trigger, "Delay")
		if h.err != nil {
			return nil, h.err
		}
		delay, err := StringToPeriod(delayStr)
		if err != nil {
			return nil, fmt.Errorf("error parsing IRegistrationTrigger object: error parsing Delay field: %w", err)
		}

		registrationTrigger := RegistrationTrigger{
			TaskTrigger: taskTriggerObj,
			Delay:       delay,
		}

		return registrationTrigger, nil
	case TASK_TRIGGER_TIME:
		randomDelayStr := h.getString(trigger, "RandomDelay")
		if h.err != nil {
			return nil, h.err
		}
		randomDelay, err := StringToPeriod(randomDelayStr)
		if err != nil {
			return nil, fmt.Errorf("error parsing ITimeTrigger object: error parsing RandomDelay field: %w", err)
		}

		timetrigger := TimeTrigger{
			TaskTrigger: taskTriggerObj,
			RandomDelay: randomDelay,
		}

		return timetrigger, nil
	case TASK_TRIGGER_WEEKLY:
		daysOfWeek := DayOfWeek(h.getInt(trigger, "DaysOfWeek"))
		randomDelayStr := h.getString(trigger, "RandomDelay")
		weeksInterval := WeekInterval(h.getInt(trigger, "WeeksInterval"))
		if h.err != nil {
			return nil, h.err
		}
		randomDelay, err := StringToPeriod(randomDelayStr)
		if err != nil {
			return nil, fmt.Errorf("error parsing IWeeklyTrigger object: error parsing RandomDelay field: %w", err)
		}

		weeklyTrigger := WeeklyTrigger{
			TaskTrigger:  taskTriggerObj,
			DaysOfWeek:   daysOfWeek,
			RandomDelay:  randomDelay,
			WeekInterval: weeksInterval,
		}

		return weeklyTrigger, nil
	case TASK_TRIGGER_SESSION_STATE_CHANGE:
		delayStr := h.getString(trigger, "Delay")
		stateChange := TaskSessionStateChangeType(h.getInt(trigger, "StateChange"))
		userID := h.getString(trigger, "UserId")
		if h.err != nil {
			return nil, h.err
		}
		delay, err := StringToPeriod(delayStr)
		if err != nil {
			return nil, fmt.Errorf("error parsing ISessionStateChangeTrigger object: error parsing Delay field: %w", err)
		}

		sessionStateChangeTrigger := SessionStateChangeTrigger{
			TaskTrigger: taskTriggerObj,
			Delay:       delay,
			StateChange: stateChange,
			UserId:      userID,
		}

		return sessionStateChangeTrigger, nil
	case TASK_TRIGGER_CUSTOM_TRIGGER_01:
		customTrigger := CustomTrigger{
			TaskTrigger: taskTriggerObj,
		}

		return customTrigger, nil

	default:
		return nil, errors.New("unsupported ITrigger type")
	}
}

var oleAutomationEpoch = time.Date(1899, time.December, 30, 0, 0, 0, 0, time.UTC)

func variantTimeOrZero(v *ole.VARIANT) time.Time {
	if v == nil || v.VT != ole.VT_DATE {
		return time.Time{}
	}

	return oleDateToTime(math.Float64frombits(uint64(v.Val)))
}

func oleDateToTime(value float64) time.Time {
	if value == 0 || math.IsNaN(value) || math.IsInf(value, 0) {
		return time.Time{}
	}

	const day = 24 * time.Hour
	days, frac := math.Modf(value)
	dayDuration := time.Duration(int64(days)) * day
	fracDuration := time.Duration(frac * float64(day))

	return oleAutomationEpoch.Add(dayDuration + fracDuration)
}
