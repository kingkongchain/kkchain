# kkchain
king kong chain


1、build
go build

2、run
only for local test, and there are some startup parameter：
-k ---- file name，store the private key of peer
-p ---- port number, 9998 as default
-m ---- miner switch,true for start, false as default

（1）single node for mining
./kkchain -k node1.key -p 9998 -m true

（2）multi node for mining
./kkchain -k node1.key -p 9998 -m true
./kkchain -k node2.key -p 9999 -m true
...

***** note *****
for multi node mining, the peer with port 9998 should specify "node1.key" if you haven't modify relational code in main.go