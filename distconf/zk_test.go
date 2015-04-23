package distconf

import (
	"errors"
	"testing"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/samuel/go-zookeeper/zk"
	"github.com/signalfx/golib/zkplus/zktest"
	"github.com/stretchr/testify/assert"
)

func TestZkConf(t *testing.T) {
	log.Info("TestZkConf")
	zkServer := zktest.New()
	z, err := Zk(ZkConnectorFunc(func() (ZkConn, <-chan zk.Event, error) {
		return zkServer.Connect()
	}))
	defer z.Close()
	assert.NoError(t, err)

	b, err := z.Get("TestZkConf")
	assert.NoError(t, err)
	assert.Nil(t, b)

	assert.NoError(t, z.Write("TestZkConf", nil))

	signalChan := make(chan struct{}, 4)
	z.(Dynamic).Watch("TestZkConf", backingCallbackFunction(func(S string) {
		assert.Equal(t, "TestZkConf", S)
		signalChan <- struct{}{}
	}))

	// The write should work and I should get a single signal on the chan
	log.Info("Doing write 1")
	assert.NoError(t, z.Write("TestZkConf", []byte("newval")))
	b, err = z.Get("TestZkConf")
	assert.NoError(t, err)
	assert.Equal(t, []byte("newval"), b)
	<-signalChan

	// Should send another signal
	log.Info("Doing write 2")
	assert.NoError(t, z.Write("TestZkConf", []byte("newval_v2")))
	//	b, err = z.Get("TestZkConf")
	//	assert.NoError(t, err)
	//	assert.Equal(t, []byte("newval_v2"), b)
	<-signalChan

	log.Info("Doing write 3")
	assert.NoError(t, z.Write("TestZkConf", nil))
	//	b, err = z.Get("TestZkConf")
	//	assert.NoError(t, err)
	//	assert.Nil(t, b)
	<-signalChan
}

func TestCloseNormal(t *testing.T) {
	zkServer := zktest.New()
	zkServer.ForcedErrorCheck(func(s string) error {
		return errors.New("nope")
	})

	z, err := Zk(ZkConnectorFunc(func() (ZkConn, <-chan zk.Event, error) {
		return zkServer.Connect()
	}))
	assert.NoError(t, err)

	z.Close()

	// Should not deadlock
	<-z.(*zkConfig).shouldQuit
}

func TestCloseQuitChan(t *testing.T) {
	zkServer := zktest.New()
	zkServer.ForcedErrorCheck(func(s string) error {
		return errors.New("nope")
	})

	z, err := Zk(ZkConnectorFunc(func() (ZkConn, <-chan zk.Event, error) {
		return zkServer.Connect()
	}))
	assert.NoError(t, err)

	// Should not deadlock
	close(z.(*zkConfig).shouldQuit)

	// Give drain() loop time to exit, for code coverage
	time.Sleep(time.Millisecond * 100)
}

func TestZkConfErrors(t *testing.T) {
	zkServer := zktest.New()
	zkServer.ForcedErrorCheck(func(s string) error {
		return errors.New("nope")
	})
	zkServer.ChanTimeout = time.Millisecond * 10

	z, err := Zk(ZkConnectorFunc(func() (ZkConn, <-chan zk.Event, error) {
		return zkServer.Connect()
	}))
	defer z.Close()
	assert.NoError(t, err)

	_, err = z.Get("TestZkConfErrors")
	assert.Error(t, err)

	assert.Error(t, z.(Dynamic).Watch("TestZkConfErrors", nil))
	assert.Error(t, z.Write("TestZkConfErrors", nil))
	assert.Error(t, z.(*zkConfig).reregisterWatch("TestZkConfErrors"))

	z.(*zkConfig).conn.Close()

	//	zkp.GlobalChan <- zk.Event{
	//		State: zk.StateDisconnected,
	//	}
	//	// Let the thread switch back to get code coverage
	time.Sleep(10 * time.Millisecond)
	//	zkp.Close()
}

func TestErrorLoader(t *testing.T) {
	_, err := Zk(ZkConnectorFunc(func() (ZkConn, <-chan zk.Event, error) {
		return nil, nil, errors.New("nope")
	}))
	assert.Error(t, err)
}