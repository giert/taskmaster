//go:build windows
// +build windows

package taskmaster

import (
	"fmt"

	ole "github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
)

func fillDefinitionObj(definition Definition, definitionObj *ole.IDispatch) error {
	h := &oleHelper{}

	actionsObj := h.getObject(definitionObj, "Actions")
	if h.err != nil {
		return h.err
	}
	defer actionsObj.Release()
	h.put(actionsObj, "Context", definition.Context)
	if h.err != nil {
		return h.err
	}
	if err := fillActionsObj(definition.Actions, actionsObj); err != nil {
		return fmt.Errorf("error filling IAction objects: %w", err)
	}

	h.put(definitionObj, "Data", definition.Data)
	if h.err != nil {
		return h.err
	}

	principalObj := h.getObject(definitionObj, "Principal")
	if h.err != nil {
		return h.err
	}
	defer principalObj.Release()
	if err := fillPrincipalObj(definition.Principal, principalObj); err != nil {
		return fmt.Errorf("error filling IPrincipal object: %w", err)
	}

	regInfoObj := h.getObject(definitionObj, "RegistrationInfo")
	if h.err != nil {
		return h.err
	}
	defer regInfoObj.Release()
	if err := fillRegistrationInfoObj(definition.RegistrationInfo, regInfoObj); err != nil {
		return fmt.Errorf("error filling IRegistrationInfo object: %w", err)
	}

	settingsObj := h.getObject(definitionObj, "Settings")
	if h.err != nil {
		return h.err
	}
	defer settingsObj.Release()
	if err := fillTaskSettingsObj(definition.Settings, settingsObj); err != nil {
		return fmt.Errorf("error filling ITaskSettings object: %w", err)
	}

	triggersObj := h.getObject(definitionObj, "Triggers")
	if h.err != nil {
		return h.err
	}
	defer triggersObj.Release()
	if err := fillTaskTriggersObj(definition.Triggers, triggersObj); err != nil {
		return fmt.Errorf("error filling ITrigger objects: %w", err)
	}

	return nil
}

func fillActionsObj(actions []Action, actionsObj *ole.IDispatch) error {
	for _, action := range actions {
		if err := fillAction(actionsObj, action); err != nil {
			return err
		}
	}

	return nil
}

func fillAction(actionsObj *ole.IDispatch, action Action) error {
	actionType := action.GetType()
	res, err := oleutil.CallMethod(actionsObj, "Create", uint(actionType))
	if err != nil {
		return fmt.Errorf("error creating IAction object: %w", getTaskSchedulerError(err))
	}
	actionObj := res.ToIDispatch()
	defer actionObj.Release()

	h := &oleHelper{}
	h.put(actionObj, "Id", action.GetID())

	switch actionType {
	case TASK_ACTION_EXEC:
		execAction := action.(ExecAction)
		exeActionObj := h.query(actionObj, ole.NewGUID("{4c3d624d-fd6b-49a3-b9b7-09cb3cd3f047}"))
		if h.err != nil {
			return h.err
		}
		defer exeActionObj.Release()

		h.put(exeActionObj, "Arguments", execAction.Args)
		h.put(exeActionObj, "Path", execAction.Path)
		h.put(exeActionObj, "WorkingDirectory", execAction.WorkingDir)
	case TASK_ACTION_COM_HANDLER:
		comHandlerAction := action.(ComHandlerAction)
		comHandlerActionObj := h.query(actionObj, ole.NewGUID("{6d2fd252-75c5-4f66-90ba-2a7d8cc3039f}"))
		if h.err != nil {
			return h.err
		}
		defer comHandlerActionObj.Release()

		h.put(comHandlerActionObj, "ClassId", comHandlerAction.ClassID)
		h.put(comHandlerActionObj, "Data", comHandlerAction.Data)
	}

	return h.err
}

func fillPrincipalObj(principal Principal, principalObj *ole.IDispatch) error {
	h := &oleHelper{}
	h.put(principalObj, "DisplayName", principal.Name)
	h.put(principalObj, "GroupId", principal.GroupID)
	h.put(principalObj, "Id", principal.ID)
	h.put(principalObj, "LogonType", uint(principal.LogonType))
	h.put(principalObj, "RunLevel", uint(principal.RunLevel))
	h.put(principalObj, "UserId", principal.UserID)
	return h.err
}

func fillRegistrationInfoObj(regInfo RegistrationInfo, regInfoObj *ole.IDispatch) error {
	h := &oleHelper{}
	h.put(regInfoObj, "Author", regInfo.Author)
	h.put(regInfoObj, "Date", TimeToTaskDate(regInfo.Date))
	h.put(regInfoObj, "Description", regInfo.Description)
	h.put(regInfoObj, "Documentation", regInfo.Documentation)
	h.put(regInfoObj, "SecurityDescriptor", regInfo.SecurityDescriptor)
	h.put(regInfoObj, "Source", regInfo.Source)
	h.put(regInfoObj, "URI", regInfo.URI)
	h.put(regInfoObj, "Version", regInfo.Version)
	return h.err
}

func fillTaskSettingsObj(settings TaskSettings, settingsObj *ole.IDispatch) error {
	h := &oleHelper{}
	h.put(settingsObj, "AllowDemandStart", settings.AllowDemandStart)
	h.put(settingsObj, "AllowHardTerminate", settings.AllowHardTerminate)
	h.put(settingsObj, "Compatibility", uint(settings.Compatibility))
	h.put(settingsObj, "DeleteExpiredTaskAfter", settings.DeleteExpiredTaskAfter)
	h.put(settingsObj, "DisallowStartIfOnBatteries", settings.DontStartOnBatteries)
	h.put(settingsObj, "Enabled", settings.Enabled)
	h.put(settingsObj, "ExecutionTimeLimit", PeriodToString(settings.TimeLimit))
	h.put(settingsObj, "Hidden", settings.Hidden)

	idlesettingsObj := h.getObject(settingsObj, "IdleSettings")
	if h.err != nil {
		return h.err
	}
	defer idlesettingsObj.Release()
	h.put(idlesettingsObj, "IdleDuration", PeriodToString(settings.IdleSettings.IdleDuration))
	h.put(idlesettingsObj, "RestartOnIdle", settings.IdleSettings.RestartOnIdle)
	h.put(idlesettingsObj, "StopOnIdleEnd", settings.IdleSettings.StopOnIdleEnd)
	h.put(idlesettingsObj, "WaitTimeout", PeriodToString(settings.IdleSettings.WaitTimeout))

	h.put(settingsObj, "MultipleInstances", uint(settings.MultipleInstances))

	networksettingsObj := h.getObject(settingsObj, "NetworkSettings")
	if h.err != nil {
		return h.err
	}
	defer networksettingsObj.Release()
	h.put(networksettingsObj, "Id", settings.NetworkSettings.ID)
	h.put(networksettingsObj, "Name", settings.NetworkSettings.Name)

	h.put(settingsObj, "Priority", settings.Priority)
	h.put(settingsObj, "RestartCount", settings.RestartCount)
	h.put(settingsObj, "RestartInterval", PeriodToString(settings.RestartInterval))
	h.put(settingsObj, "RunOnlyIfIdle", settings.RunOnlyIfIdle)
	h.put(settingsObj, "RunOnlyIfNetworkAvailable", settings.RunOnlyIfNetworkAvailable)
	h.put(settingsObj, "StartWhenAvailable", settings.StartWhenAvailable)
	h.put(settingsObj, "StopIfGoingOnBatteries", settings.StopIfGoingOnBatteries)
	h.put(settingsObj, "WakeToRun", settings.WakeToRun)

	return h.err
}

func fillTaskTriggersObj(triggers []Trigger, triggersObj *ole.IDispatch) error {
	for _, trigger := range triggers {
		if err := fillTrigger(triggersObj, trigger); err != nil {
			return err
		}
	}

	return nil
}

func fillTrigger(triggersObj *ole.IDispatch, trigger Trigger) error {
	res, err := oleutil.CallMethod(triggersObj, "Create", uint(trigger.GetType()))
	if err != nil {
		return fmt.Errorf("error creating ITrigger object: %w", getTaskSchedulerError(err))
	}
	triggerObj := res.ToIDispatch()
	defer triggerObj.Release()

	h := &oleHelper{}
	h.put(triggerObj, "Enabled", trigger.GetEnabled())
	h.put(triggerObj, "EndBoundary", TimeToTaskDate(trigger.GetEndBoundary()))
	h.put(triggerObj, "ExecutionTimeLimit", PeriodToString(trigger.GetExecutionTimeLimit()))
	h.put(triggerObj, "Id", trigger.GetID())

	repetitionObj := h.getObject(triggerObj, "Repetition")
	if h.err != nil {
		return h.err
	}
	defer repetitionObj.Release()
	h.put(repetitionObj, "Duration", PeriodToString(trigger.GetRepetitionDuration()))
	h.put(repetitionObj, "Interval", PeriodToString(trigger.GetRepetitionInterval()))
	h.put(repetitionObj, "StopAtDurationEnd", trigger.GetStopAtDurationEnd())

	h.put(triggerObj, "StartBoundary", TimeToTaskDate(trigger.GetStartBoundary()))
	if h.err != nil {
		return h.err
	}

	switch t := trigger.(type) {
	case BootTrigger:
		bootTriggerObj := h.query(triggerObj, ole.NewGUID("{2a9c35da-d357-41f4-bbc1-207ac1b1f3cb}"))
		if h.err != nil {
			return h.err
		}
		defer bootTriggerObj.Release()

		h.put(bootTriggerObj, "Delay", PeriodToString(t.Delay))
	case DailyTrigger:
		dailyTriggerObj := h.query(triggerObj, ole.NewGUID("{126c5cd8-b288-41d5-8dbf-e491446adc5c}"))
		if h.err != nil {
			return h.err
		}
		defer dailyTriggerObj.Release()

		h.put(dailyTriggerObj, "DaysInterval", uint(t.DayInterval))
		h.put(dailyTriggerObj, "RandomDelay", PeriodToString(t.RandomDelay))
	case EventTrigger:
		eventTriggerObj := h.query(triggerObj, ole.NewGUID("{d45b0167-9653-4eef-b94f-0732ca7af251}"))
		if h.err != nil {
			return h.err
		}
		defer eventTriggerObj.Release()

		h.put(eventTriggerObj, "Delay", PeriodToString(t.Delay))
		h.put(eventTriggerObj, "Subscription", t.Subscription)
		valueQueriesObj := h.getObject(eventTriggerObj, "ValueQueries")
		if h.err != nil {
			return h.err
		}
		defer valueQueriesObj.Release()

		for name, value := range t.ValueQueries {
			if _, err := oleutil.CallMethod(valueQueriesObj, "Create", name, value); err != nil {
				return fmt.Errorf("error creating value %s: %w", name, getTaskSchedulerError(err))
			}
		}
	case IdleTrigger:
		idleTriggerObj := h.query(triggerObj, ole.NewGUID("{d537d2b0-9fb3-4d34-9739-1ff5ce7b1ef3}"))
		if h.err != nil {
			return h.err
		}
		defer idleTriggerObj.Release()
	case LogonTrigger:
		logonTriggerObj := h.query(triggerObj, ole.NewGUID("{72dade38-fae4-4b3e-baf4-5d009af02b1c}"))
		if h.err != nil {
			return h.err
		}
		defer logonTriggerObj.Release()

		h.put(logonTriggerObj, "Delay", PeriodToString(t.Delay))
		h.put(logonTriggerObj, "UserId", t.UserID)
	case MonthlyDOWTrigger:
		monthlyDOWTriggerObj := h.query(triggerObj, ole.NewGUID("{77d025a3-90fa-43aa-b52e-cda5499b946a}"))
		if h.err != nil {
			return h.err
		}
		defer monthlyDOWTriggerObj.Release()

		h.put(monthlyDOWTriggerObj, "DaysOfWeek", uint(t.DaysOfWeek))
		h.put(monthlyDOWTriggerObj, "MonthsOfYear", uint(t.MonthsOfYear))
		h.put(monthlyDOWTriggerObj, "RandomDelay", PeriodToString(t.RandomDelay))
		h.put(monthlyDOWTriggerObj, "RunOnLastWeekOfMonth", t.RunOnLastWeekOfMonth)
		h.put(monthlyDOWTriggerObj, "WeeksOfMonth", uint(t.WeeksOfMonth))
	case MonthlyTrigger:
		monthlyTriggerObj := h.query(triggerObj, ole.NewGUID("{97c45ef1-6b02-4a1a-9c0e-1ebfba1500ac}"))
		if h.err != nil {
			return h.err
		}
		defer monthlyTriggerObj.Release()

		h.put(monthlyTriggerObj, "DaysOfMonth", uint(t.DaysOfMonth))
		h.put(monthlyTriggerObj, "MonthsOfYear", uint(t.MonthsOfYear))
		h.put(monthlyTriggerObj, "RandomDelay", PeriodToString(t.RandomDelay))
		h.put(monthlyTriggerObj, "RunOnLastDayOfMonth", t.RunOnLastDayOfMonth)
	case RegistrationTrigger:
		registrationTriggerObj := h.query(triggerObj, ole.NewGUID("{4c8fec3a-c218-4e0c-b23d-629024db91a2}"))
		if h.err != nil {
			return h.err
		}
		defer registrationTriggerObj.Release()

		h.put(registrationTriggerObj, "Delay", PeriodToString(t.Delay))
	case TimeTrigger:
		timeTriggerObj := h.query(triggerObj, ole.NewGUID("{b45747e0-eba7-4276-9f29-85c5bb300006}"))
		if h.err != nil {
			return h.err
		}
		defer timeTriggerObj.Release()

		h.put(timeTriggerObj, "RandomDelay", PeriodToString(t.RandomDelay))
	case WeeklyTrigger:
		weeklyTriggerObj := h.query(triggerObj, ole.NewGUID("{5038fc98-82ff-436d-8728-a512a57c9dc1}"))
		if h.err != nil {
			return h.err
		}
		defer weeklyTriggerObj.Release()

		h.put(weeklyTriggerObj, "DaysOfWeek", uint(t.DaysOfWeek))
		h.put(weeklyTriggerObj, "RandomDelay", PeriodToString(t.RandomDelay))
		h.put(weeklyTriggerObj, "WeeksInterval", uint(t.WeekInterval))
	case SessionStateChangeTrigger:
		sessionStateChangeTriggerObj := h.query(triggerObj, ole.NewGUID("{754da71b-4385-4475-9dd9-598294fa3641}"))
		if h.err != nil {
			return h.err
		}
		defer sessionStateChangeTriggerObj.Release()

		h.put(sessionStateChangeTriggerObj, "Delay", PeriodToString(t.Delay))
		h.put(sessionStateChangeTriggerObj, "StateChange", uint(t.StateChange))
		h.put(sessionStateChangeTriggerObj, "UserId", t.UserId)
		// CustomTrigger (TASK_TRIGGER_CUSTOM_TRIGGER_01) has no public CLSID
		// and cannot be created programmatically, so it is intentionally not
		// handled here.
	}

	return h.err
}
