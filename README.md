============================ GIERT'S FORK NOTES ============================

Maintained, hardened fork. Compared with upstream:
  - Requires Go 1.22+; dependencies updated (go-ole, and the deprecated
    rickb777/date replaced with the maintained rickb777/period).
  - Parsing/COM errors are returned instead of panicking (no recover() needed).
  - Assorted bug fixes and a couple of API additions.

Note: a TaskService is not goroutine-safe — create, use, and Disconnect it on
the same goroutine. Releases are tagged; pin a version (or commit).

I also added a list of gotchas that have bitten me over the years at the bottom of the readme.

=========================== /GIERT'S FORK NOTES ===========================

[![CI](https://github.com/giert/taskmaster/actions/workflows/ci.yml/badge.svg)](https://github.com/giert/taskmaster/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/giert/taskmaster)](https://goreportcard.com/report/github.com/giert/taskmaster)
[![PkgGoDev](https://pkg.go.dev/badge/github.com/giert/taskmaster)](https://pkg.go.dev/github.com/giert/taskmaster)

# taskmaster
Windows Task Scheduler Library for Go

**NOTE:** the API is *not* stable, I reserve the right to change it before v1.0. Task Scheduler is complex, and it is difficult to create a sane, useable interface for it. I would highly encourage you to make use of Go modules and pin a specific commit.

![taskmaster villain](img/taskmaster.jpg "Taskmaster")

# What is taskmaster?

Taskmaster is a library for managing Scheduled Tasks in Windows. It allows you to easily create, modify, delete, execute, kill, and view scheduled tasks, on your local machine or on a remote one. It provides much more speed and power than using the native Task Scheduler GUI in Windows, and the Scheduled Task Powershell cmdlets.

Because taskmaster interfaces directly with Task Scheduler COM objects, it allows you to do things you can't do with the Task Scheduler GUI or Powershell cmdlets. COM handler task actions can be viewed, manipulated, and created, more settings can be used when creating or modifying scheduled tasks, etc. Taskmaster exposes the full potential of Windows Scheduled Tasks in a clean, simple interface.

# Documentation

As I was researching the Task Scheduler COM interface more and more, I quickly realized just how complex and confusing all the different parts of Task Scheduler are. So I set out to concisely copy the documentation from MSDN into taskmaster, but also consolidate it and add information that is buried in the depths of MSDN. This should make using both taskmaster and the existing Task Scheduler tools easier, having a ton of information and links to Task Scheduler internals available via GoDocs. If you find info that I missed, feel free to submit an issue or better yet open a PR :)

There are a lot of hidden gotchas and quirks within Task Scheduler, so I would *highly* recommend perusing the official docs before attempting really anything with this library on [MSDN](https://docs.microsoft.com/en-us/windows/win32/taskschd/task-scheduler-start-page).

## Automatic Maintenance

On Windows 8 / Server 2012 and later, opt a task into Windows' maintenance schedule
with `Settings.MaintenanceSettings`. This requires `TASK_COMPATIBILITY_V2_2` or
later and automatically enables `UseUnifiedSchedulingEngine` for that task.

```go
// taskmaster is github.com/giert/taskmaster; period is github.com/rickb777/period.
def := taskmaster.DefaultDefinition()
def.Settings.Compatibility = taskmaster.TASK_COMPATIBILITY_V2_2
def.Settings.MaintenanceSettings = &taskmaster.MaintenanceSettings{
    Period:    period.NewYMD(0, 0, 2),  // at least one day
    Deadline:  period.NewYMD(0, 0, 14), // optional; must be longer than Period
    Exclusive: false,
}
def.AddAction(taskmaster.ExecAction{Path: `C:\Program Files\MyApp\maintenance.exe`})
// Register def with TaskService.CreateTask, as with an ordinary definition.
```

A zero `Deadline` omits emergency maintenance. A nil `MaintenanceSettings` leaves
the task outside Automatic Maintenance; setting it to nil when updating a task
removes the configuration. Ordinary tasks do not require the newer COM interface
when written. Reading a task without maintenance settings returns nil.

Durations follow XML Schema syntax: use days instead of weeks, and whole numbers
except for fractional seconds. The library checks the one-day minimum and
unambiguous deadline ordering without overflowing `time.Duration`; Windows checks
calendar-dependent comparisons (such as one month versus 31 days) at registration.
The unified scheduling engine also restricts which combinations of triggers and
settings Windows accepts; registration errors are returned to the caller.

Windows chooses maintenance opportunities, normally while the computer is idle
and on AC power. This does not promise execution at a chosen clock time or a
particular wake-from-sleep behavior. Work should tolerate interruption and resume
its own progress. See [Microsoft's Automatic Maintenance guide](https://learn.microsoft.com/en-us/windows/win32/taskschd/task-maintenence).

====================== GIERT'S TASK SCHEDULER GOTCHAS ======================

This library faithfully exposes most of Windows Task Scheduler's rough edges rather than hiding them. A few to be extra aware of, maybe you can avoid my mistakes and wasted time:

- **Tasks don't run on battery by default.** `Settings.DontStartOnBatteries` and `StopIfGoingOnBatteries` default to `true` (the Task Scheduler default), so a task can silently never start on a laptop (or suddenly start when the charger is plugged in) — set both to `false` to allow it.
- **A manual `Run`/`RunEx` still honours conditions** ("don't start on battery", "only if idle", …), so it may silently do nothing; pass `TASK_RUN_IGNORE_CONSTRAINTS` to force a run.
- **`RunLevel: TASK_RUNLEVEL_HIGHEST` requires the creator to be elevated**, or registration fails with "Access is denied".
- **Local accounts usually need the machine name** in `Principal.UserID` (`COMPUTERNAME\user`, not a bare username), or registration fails with "No mapping between account names and security IDs was done".
- **Repetition:** `RepetitionInterval` must be at least one minute, and a non-zero `RepetitionDuration` must be longer than the interval. To repeat indefinitely, leave `RepetitionDuration` zero.
- **`DeleteExpiredTaskAfter` only takes effect if a trigger has an `EndBoundary`.**
- **`Settings.Compatibility`** must match the features used — multiple triggers or actions need `TASK_COMPATIBILITY_V2` or newer (the default here).

===================== /GIERT'S TASK SCHEDULER GOTCHAS =====================
