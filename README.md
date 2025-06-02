# Rapid7 InsightOps logrus hook

## Basic setup

Top two lines are optional, configure `logrus` as you please.

```go
logrus.SetLevel(logrus.DebugLevel)
logrus.SetFormatter(&logrus.JSONFormatter{})

hook, err := New(
    os.Getenv("InsightToken"),
    "eu",
    &Opts{
        Priority: logrus.InfoLevel,
    },
)
if err != nil {
    panic(err)
}
logrus.AddHook(hook)

// ... other stuff you have in your app  ...

// When shutting down your application, flush and close the connection pool
hook.FlushAndClose()
```

## Note on Connection Pooling

This client uses a connection pool for efficiency. It is important to call `FlushAndClose()` on your `InsightOpsHook` instance before your application exits to ensure all connections are properly closed and all logs are sent.
