package lifecycle

import (
	"context"
	"fmt"
	"testing"
)

func BenchmarkManager_StartStop_Serial(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		m := NewManager(WithLogger(NopLogger()))
		for j := 0; j < 100; j++ {
			m.AddTask(NewFuncTask(fmt.Sprintf("task-%d", j),
				func(ctx context.Context) error { return nil },
				func(ctx context.Context) error { return nil },
			))
		}
		_ = m.Start(context.Background())
		_ = m.Stop(context.Background())
	}
}

func BenchmarkManager_StartStop_Parallel(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		m := NewManager(WithLogger(NopLogger()), WithExecutor(ExecutorParallel))
		for j := 0; j < 100; j++ {
			m.AddTask(NewFuncTask(fmt.Sprintf("task-%d", j),
				func(ctx context.Context) error { return nil },
				func(ctx context.Context) error { return nil },
			))
		}
		_ = m.Start(context.Background())
		_ = m.Stop(context.Background())
	}
}

func BenchmarkSerialExecutor_StartStop(b *testing.B) {
	b.ReportAllocs()
	exec := newSerialExecutor(NopLogger(), true)
	tasks := make([]Task, 100)
	for j := 0; j < 100; j++ {
		tasks[j] = NewFuncTask(fmt.Sprintf("task-%d", j),
			func(ctx context.Context) error { return nil },
			func(ctx context.Context) error { return nil },
		)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = exec.Start(context.Background(), tasks)
		_ = exec.Stop(context.Background(), tasks)
	}
}

func BenchmarkParallelExecutor_StartStop(b *testing.B) {
	b.ReportAllocs()
	exec := newParallelExecutor(NopLogger(), true)
	tasks := make([]Task, 100)
	for j := 0; j < 100; j++ {
		tasks[j] = NewFuncTask(fmt.Sprintf("task-%d", j),
			func(ctx context.Context) error { return nil },
			func(ctx context.Context) error { return nil },
		)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = exec.Start(context.Background(), tasks)
		_ = exec.Stop(context.Background(), tasks)
	}
}

func BenchmarkManager_WithHooks(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		m := NewManager(WithLogger(NopLogger()))
		m.OnStart(func(ctx context.Context) error { return nil })
		m.OnReady(func(ctx context.Context) error { return nil })
		m.OnStopping(func(ctx context.Context) error { return nil })
		m.OnStopped(func(ctx context.Context) error { return nil })

		for j := 0; j < 50; j++ {
			m.AddTask(NewFuncTask(fmt.Sprintf("task-%d", j),
				func(ctx context.Context) error { return nil },
				func(ctx context.Context) error { return nil },
			))
		}
		_ = m.Start(context.Background())
		_ = m.Stop(context.Background())
	}
}

func BenchmarkManager_WithMetrics(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		m := NewManager(
			WithLogger(NopLogger()),
			WithMetricsHook(func(e MetricEvent) {}),
		)
		for j := 0; j < 50; j++ {
			m.AddTask(NewFuncTask(fmt.Sprintf("task-%d", j),
				func(ctx context.Context) error { return nil },
				func(ctx context.Context) error { return nil },
			))
		}
		_ = m.Start(context.Background())
		_ = m.Stop(context.Background())
	}
}

func BenchmarkFuncTask_Creation(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = NewFuncTask("bench",
			func(ctx context.Context) error { return nil },
			func(ctx context.Context) error { return nil },
			WithTaskPriority(5),
		)
	}
}
