compile:
	go build -o ./bin/nonat ./cmd/cni/

clean:
	CNI_PATH=./bin NETCONFPATH=./tools ./tools/cnitool del nonat-conf /var/run/netns/cni-1234 || true
	ip netns del cni-1234 || true
	rm -r ./bin/ || true

cnitool:
	cp ./tools/host-local ./bin/
	ip netns add cni-1234
	CNI_PATH=./bin/ NETCONFPATH=./tools ./tools/cnitool add nonat-conf /var/run/netns/cni-1234
