package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"strings"

	bhost "github.com/libp2p/go-libp2p/p2p/host/basic"

	host "github.com/libp2p/go-libp2p-host"
	inet "github.com/libp2p/go-libp2p-net"
	net "github.com/libp2p/go-libp2p-net"
	peer "github.com/libp2p/go-libp2p-peer"
	pstore "github.com/libp2p/go-libp2p-peerstore"
	swarm "github.com/libp2p/go-libp2p-swarm"
	testutil "github.com/libp2p/go-testutil"
	ma "github.com/multiformats/go-multiaddr"
)

// create a 'Host' with a random peer to listen on the given address
func makeDummyHost(listen string, secio bool) (host.Host, error) {
	addr, err := ma.NewMultiaddr(listen)
	if err != nil {
		return nil, err
	}

	ps := pstore.NewPeerstore()
	var pid peer.ID

	if secio {
		ident, err := testutil.RandIdentity()
		if err != nil {
			return nil, err
		}

		ident.PrivateKey()
		ps.AddPrivKey(ident.ID(), ident.PrivateKey())
		ps.AddPubKey(ident.ID(), ident.PublicKey())
		pid = ident.ID()
	} else {
		fakepid, err := testutil.RandPeerID()
		if err != nil {
			return nil, err
		}
		pid = fakepid
	}

	ctx := context.Background()

	// create a new swarm to be used by the service host
	netw, err := swarm.NewNetwork(ctx, []ma.Multiaddr{addr}, pid, ps, nil)
	if err != nil {
		return nil, err
	}

	log.Printf("I am %s/ipfs/%s\n", addr, pid.Pretty())
	return bhost.New(netw), nil
}

func main() {
	listenF := flag.Int("l", 0, "wait for incoming connections")
	target := flag.String("d", "", "target peer to dial")
	secio := flag.Bool("secio", false, "enable secio")

	flag.Parse()

	listenaddr := fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", *listenF)

	ha, err := makeDummyHost(listenaddr, *secio)
	if err != nil {
		log.Fatal(err)
	}

	// Set a stream handler on host A
	ha.SetStreamHandler("/echo/1.0.0", func(s net.Stream) {
		log.Println("got a new stream")
		doEcho(s)
		defer s.Close()
	})

	if *target == "" {
		log.Println("listening for connections")
		select {} // hang forever
	}

	ipfsaddr, err := ma.NewMultiaddr(*target)
	if err != nil {
		log.Fatalln(err)
	}

	pid, err := ipfsaddr.ValueForProtocol(ma.P_IPFS)
	if err != nil {
		log.Fatalln(err)
	}

	peerid, err := peer.IDB58Decode(pid)
	if err != nil {
		log.Fatalln(err)
	}

	tptaddr := strings.Split(ipfsaddr.String(), "/ipfs/")[0]
	tptmaddr, err := ma.NewMultiaddr(tptaddr)
	if err != nil {
		log.Fatalln(err)
	}

	pi := pstore.PeerInfo{
		ID:    peerid,
		Addrs: []ma.Multiaddr{tptmaddr},
	}

	log.Println("connecting to target")
	err = ha.Connect(context.Background(), pi)
	if err != nil {
		log.Fatalln(err)
	}

	log.Println("opening stream")
	// make a new stream from host B to host A
	// it should be handled on host A by the handler we set
	s, err := ha.NewStream(context.Background(), peerid, "/echo/1.0.0")
	if err != nil {
		log.Fatalln(err)
	}

	_, err = s.Write([]byte("Hello, world!"))
	if err != nil {
		log.Fatalln(err)
	}

	out, err := ioutil.ReadAll(s)
	if err != nil {
		log.Fatalln(err)
	}

	log.Printf("read reply: %q\n", out)
}

func doEcho(s inet.Stream) {
	buf := make([]byte, 1024)
	n, err := s.Read(buf)
	if err != nil {
		log.Println(err)
		return
	}

	log.Printf("read request: %q\n", buf[:n])
	_, err = s.Write(buf[:n])
	if err != nil {
		log.Println(err)
		return
	}
}
