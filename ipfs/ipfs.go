package ipfs

import (
	"bytes"
	"context"
	"errors"
	"github.com/ipsn/go-ipfs/core"
	"github.com/ipsn/go-ipfs/core/coreapi"
	"github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-cid"
	"github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-ipfs-files"
	"github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/interface-go-ipfs-core"
	"github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/interface-go-ipfs-core/options"
	"github.com/ipsn/go-ipfs/plugin/loader"
	"idena-go/log"
	"path/filepath"
	"strconv"
	"time"
)

var (
	DefaultBufSize = 1048576
	EmptyCid       cid.Cid
)

func init() {
	e, _ := cid.Decode("QmdfTbBqBPQ7VNxZEYEj14VmRuZBkqFbiwReogJgS1zR1n")
	EmptyCid = e
}

type Proxy interface {
	Add(data []byte) (cid.Cid, error)
	Get(key []byte) ([]byte, error)
	Pin(key []byte) error
	Cid(data []byte) (cid.Cid, error)
}

type ipfsProxy struct {
	node *core.IpfsNode
	log  log.Logger
}

func NewIpfsProxy(config *IpfsConfig) (Proxy, error) {

	loadPlugins(config.datadir)

	if err := config.Initialize(); err != nil {
		return nil, err
	}

	logger := log.New()

	node, err := core.NewNode(context.Background(), GetNodeConfig(config.datadir))
	if err != nil {
		return nil, err
	}
	node.Repo.SetConfig(config.cfg)
	logger.Info("Ipfs initialized", "peerId", node.PeerHost.ID().Pretty())
	go watch(node)
	return &ipfsProxy{
		node: node,
		log:  logger,
	}, nil
}

func watch(node *core.IpfsNode) {
	api, _ := coreapi.NewCoreAPI(node)
	logger := log.New("component", "ipfs watch")
	for {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
		info, err := api.Swarm().Peers(ctx)
		cancel()
		logger.Info("peers info", "err", err)
		for index, i := range info {
			logger.Info(strconv.Itoa(index), "id", i.ID().String(), "addr", i.Address().String())
		}
		time.Sleep(time.Second * 5)
	}
}

func (p ipfsProxy) Add(data []byte) (cid.Cid, error) {
	if len(data) == 0 {
		return EmptyCid, nil
	}
	api, _ := coreapi.NewCoreAPI(p.node)

	file := files.NewBytesFile(data)
	path, err := api.Unixfs().Add(context.Background(), file)

	if err != nil {
		return cid.Cid{}, err
	}
	p.log.Info("Add ipfs data", "cid", path.Cid().String())
	return path.Cid(), nil
}

func (p ipfsProxy) Get(key []byte) ([]byte, error) {
	c, err := cid.Cast(key)
	if err != nil {
		return nil, err
	}
	if c == EmptyCid {
		return []byte{}, nil
	}
	api, _ := coreapi.NewCoreAPI(p.node)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	f, err := api.Unixfs().Get(ctx, iface.IpfsPath(c))

	if err != nil {
		p.log.Error("fail to read from ipfs", "cid", c.String(), "err", err)
		return nil, err
	}

	file := files.ToFile(f)

	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(file)

	if err != nil {
		return nil, err
	}
	p.log.Info("read data from ipfs", "cid", c.String())
	return buf.Bytes(), nil
}

func (p ipfsProxy) Pin(key []byte) error {
	api, _ := coreapi.NewCoreAPI(p.node)

	c, err := cid.Cast(key)
	if err != nil {
		return err
	}

	return api.Pin().Add(context.Background(), iface.IpfsPath(c))
}

func (p ipfsProxy) Cid(data []byte) (cid.Cid, error) {

	if len(data) == 0 {
		return EmptyCid, nil
	}

	api, _ := coreapi.NewCoreAPI(p.node)

	file := files.NewBytesFile(data)
	path, _ := api.Unixfs().Add(context.Background(), file, options.Unixfs.HashOnly(true))
	return path.Cid(), nil
}

func loadPlugins(ipfsPath string) (*loader.PluginLoader, error) {
	pluginpath := filepath.Join(ipfsPath, "plugins")

	var plugins *loader.PluginLoader
	plugins, err := loader.NewPluginLoader(pluginpath)

	if err != nil {
		log.Error("ipfs plugin loader error")
	}

	if err := plugins.Initialize(); err != nil {
		log.Error("ipfs plugin initialization error")
	}

	if err := plugins.Inject(); err != nil {
		log.Error("ipfs plugin inject error")
	}

	return plugins, nil
}

type InMemoryIpfs struct {
	values map[cid.Cid][]byte
}

func (i InMemoryIpfs) Add(data []byte) (cid.Cid, error) {
	cid, _ := i.Cid(data)
	i.values[cid] = data
	return cid, nil
}

func (i InMemoryIpfs) Get(key []byte) ([]byte, error) {
	c, err := cid.Parse(key)
	if err != nil {
		return nil, err
	}
	if v, ok := i.values[c]; ok {
		return v, nil
	}
	return nil, errors.New("not found")
}

func (InMemoryIpfs) Pin(key []byte) error {
	return nil
}

func (InMemoryIpfs) Cid(data []byte) (cid.Cid, error) {
	format := cid.V0Builder{}
	return format.Sum(data)
}
