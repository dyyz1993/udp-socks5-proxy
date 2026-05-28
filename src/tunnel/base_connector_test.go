package tunnel

import (
	"errors"
	"net"
	"sync"
	"testing"
)

// mockStream 用于测试的 TunnelStream mock
type mockStream struct {
	id   string
	data []byte
	mu   sync.Mutex
}

func (m *mockStream) GetStreamID() string { return m.id }
func (m *mockStream) Close() error        { return nil }
func (m *mockStream) PutData(data []byte) error {
	m.mu.Lock()
	m.data = append(m.data, data...)
	m.mu.Unlock()
	return nil
}
func (m *mockStream) GetData() ([]byte, error)      { m.mu.Lock(); defer m.mu.Unlock(); return m.data, nil }
func (m *mockStream) ServeConn(conn net.Conn) error { return nil }

func TestNewBaseConnector(t *testing.T) {
	bc := NewBaseConnector()
	if bc == nil {
		t.Fatal("NewBaseConnector returned nil")
	}
	if bc.GetConnectionID() == "" {
		t.Error("connectionID should not be empty")
	}
	if bc.state != StateInitialized {
		t.Errorf("initial state should be StateInitialized, got %d", bc.state)
	}
}

func TestBaseConnector_ConnectionID(t *testing.T) {
	bc := NewBaseConnector()
	if bc.GetConnectionID() == "" {
		t.Error("GetConnectionID returned empty")
	}
	bc.SetConnectionID("test-conn-123")
	if bc.GetConnectionID() != "test-conn-123" {
		t.Errorf("SetConnectionID failed, got %q", bc.GetConnectionID())
	}
}

func TestBaseConnector_State(t *testing.T) {
	bc := NewBaseConnector()
	if bc.IsConnected() {
		t.Error("should not be connected initially")
	}
	bc.SetState(StateConnected)
	if !bc.IsConnected() {
		t.Error("should be connected after SetState(StateConnected)")
	}
	for _, s := range []ConnectionState{StateInitialized, StateConnecting, StateDisconnecting, StateClosed, StateReconnecting} {
		bc.SetState(s)
		if bc.IsConnected() {
			t.Errorf("should not be connected for state %d", s)
		}
	}
}

func TestBaseConnector_StreamCRUD(t *testing.T) {
	bc := NewBaseConnector()
	bc.AddStream("s1", &mockStream{id: "s1"})
	got, err := bc.GetStream("s1")
	if err != nil || got.GetStreamID() != "s1" {
		t.Fatalf("GetStream(s1) failed: err=%v id=%q", err, got.GetStreamID())
	}
	if _, err := bc.GetStream("not-exist"); !errors.Is(err, ErrStreamNotFound) {
		t.Errorf("expected ErrStreamNotFound, got %v", err)
	}
	bc.RemoveStream("s1")
	if _, err := bc.GetStream("s1"); !errors.Is(err, ErrStreamNotFound) {
		t.Errorf("should be removed, got %v", err)
	}
	bc.RemoveStream("not-exist")
}

func TestBaseConnector_EmptyImplementations(t *testing.T) {
	bc := NewBaseConnector()
	if err := bc.Connect(); err != nil {
		t.Errorf("Connect: %v", err)
	}
	if err := bc.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
	if err := bc.SendData("s", nil); err != nil {
		t.Errorf("SendData: %v", err)
	}
	if id, s, err := bc.CreateStream("addr"); id != "" || s != nil || err != nil {
		t.Errorf("CreateStream: id=%q s=%v err=%v", id, s, err)
	}
	if err := bc.ProcessIncomingData(nil); err != nil {
		t.Errorf("ProcessIncomingData: %v", err)
	}
	if err := bc.Start(); err != nil {
		t.Errorf("Start: %v", err)
	}
}

func TestBaseConnector_Concurrent(t *testing.T) {
	bc := NewBaseConnector()
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			id := string(rune('a' + i%26))
			bc.AddStream(id, &mockStream{id: id})
			bc.GetStream(id)
			bc.SetState(StateConnected)
			_ = bc.IsConnected()
			bc.RemoveStream(id)
		}(i)
	}
	wg.Wait()
}
