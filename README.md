# Rapid7 InsightOps logrus hook

## Basic setup

Top two lines are optional, configure `logrus` as you please.

```go
logrus.SetLevel(logrus.DebugLevel)
logrus.SetFormatter(&logrus.JSONFormatter{})

hook, err := New(
    os.Getenv("Insight.Token"),
    "eu",
    &Opts{
        Priority: logrus.InfoLevel,
    },
)
if err != nil {
    panic(err)
}
logrus.AddHook(hook)

```
