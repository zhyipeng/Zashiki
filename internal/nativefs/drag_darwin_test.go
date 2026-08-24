//go:build darwin

package nativefs

import "testing"

func TestDropEffectFromInt(t *testing.T) {
	cases := []struct {
		op   int
		want DropEffect
	}{
		{0, 0},
		{1, DropEffectCopy},                      // NSDragOperationCopy
		{2, DropEffectMove},                      // NSDragOperationMove
		{4, DropEffectLink},                      // NSDragOperationLink
		{1 | 2, DropEffectCopy | DropEffectMove}, // Copy|Move
		{1 | 2 | 4, DropEffectCopy | DropEffectMove | DropEffectLink},
	}
	for _, c := range cases {
		if got := dropEffectFromInt(c.op); got != c.want {
			t.Errorf("dropEffectFromInt(%d) = %v, want %v", c.op, got, c.want)
		}
	}
}

func TestDarwinDragRejectsEmptyPaths(t *testing.T) {
	d := newDarwinDrag()
	if _, err := d.StartDrag(nil, 0, 0, DropEffectCopy|DropEffectMove); err == nil {
		t.Fatal("expected error for empty paths")
	}
	if _, err := d.StartDrag([]string{}, 0, 0, DropEffectCopy); err == nil {
		t.Fatal("expected error for empty paths")
	}
}

func TestDarwinDragRequiresWindowHandle(t *testing.T) {
	d := newDarwinDrag()
	windowProvider = nil
	_, err := d.StartDrag([]string{"/a.txt"}, 10, 10, DropEffectCopy)
	if err == nil {
		t.Fatal("expected error when no window handle is registered")
	}
}

func TestDarwinDragResolveIfPending(t *testing.T) {
	d := newDarwinDrag()
	id, ch := d.register()
	defer d.unregister(id)

	// 模拟拖拽结束回调
	d.resolveIfPending(2) // NSDragOperationMove
	select {
	case effect := <-ch:
		if effect != DropEffectMove {
			t.Fatalf("expected Move, got %v", effect)
		}
	default:
		t.Fatal("expected effect to be delivered")
	}
}

func TestDarwinDragResolveIfPendingIdempotent(t *testing.T) {
	d := newDarwinDrag()
	id, ch := d.register()
	defer d.unregister(id)

	d.resolveIfPending(1) // Copy
	d.resolveIfPending(2) // Move（应被忽略，等待者已移除）
	select {
	case effect := <-ch:
		if effect != DropEffectCopy {
			t.Fatalf("expected first effect Copy, got %v", effect)
		}
	default:
		t.Fatal("expected first effect to be delivered")
	}
	select {
	case <-ch:
		t.Fatal("did not expect second effect")
	default:
		// ok：第二个回调未送达
	}
}
