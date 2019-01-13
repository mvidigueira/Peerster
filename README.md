# Decentralized Search

Couse Project - Decentralized Systems Engineering 18 (EPFL)

# Run

### Install dependencies 

1. Protobuf - github.com/dedis/protobuf
2. Murmur3 (hash-function) - github.com/spaolacci/murmur3
3. Goquery (html parsing library) - github.com/PuerkitoBio/goquery
4. Stopwords - github.com/bbalet/stopwords
5. Porterstemmer (stemming library) github.com/reiver/go-porterstemmer
6. BoltDB (Persistent key-value store) github.com/etcd-io/bbolt
7. Gcache (LRU cache) https://github.com/bluele/gcache

All of the above dependencies are go gettable.

### Setup a DHT network


The following code sets up a DHT network consiting of three nodes. Every network needs a bootstrap node (node A) which other nodes need to have knowledge about at start up in order to join the network (node B & C).  

`./Peerster -UIPort=4000 -gossipAddr=127.0.0.1:7000 -name=A`

`./Peerster -UIPort=4001 -gossipAddr=127.0.0.1:7001 -name=B -boot=127.0.0.1:7000`

`./Peerster -UIPort=4002 -gossipAddr=127.0.0.1:7002 -name=C -boot=127.0.0.1:7000`

### Setup crawling

In order to limit the scope of this project, we have constrained our webcrawling to wikipedia articles. Our application has a hard coded entry point where crawling starts (/wiki/Sweden).  

We only want one node to parse the entry point and hence we give the flag `-crawlLeader` to one of the nodes in the network. All nodes starts listening for nodes to crawl by default but be passive until the receieve their first batch of URLs from the crawl leader which parses the root page.

The following code sets up a 3 node DHT & Crawling network:

`./Peerster -UIPort=4000 -gossipAddr=127.0.0.1:7000 -name=A`

`./Peerster -UIPort=4001 -gossipAddr=127.0.0.1:7001 -name=B -boot=127.0.0.1:7000`

`./Peerster -UIPort=4002 -gossipAddr=127.0.0.1:7002 -name=C -boot=127.0.0.1:7000 -crawlLeader`

The crawler saves keywords together with their frequencies for each document in the DHT. 

### Sending queries

#### store a value in the DHT

`./client -UIPort=4000 -store=key:value`

Where key is a string and value can either be a string or a path to a file on the local file system. The key can be of arbitrary length and will be transformed to a 20byte hash. 

#### look up a value

`./client -UIPort=4000 -lookupKey=key`

Example of query of webcrawler data: 

`./client -UIPort=4000 -lookupKey=Sweden`

The above query will return the URL of every crawled document in the DHT which contain the keyword **sweden** together with the number of occurances of that keyword in the document.

### UI

The web UI for the search functionality can be found on port 80 of localhost.

### Encryption

Enable AES encryption for all dht store/lookup operation by providing the -encryptDHSOperations flag. The AES encryption keys will be negotiated automatically using Diffie-Hellman key exchange.

`./Peerster -UIPort=4002 -gossipAddr=127.0.0.1:7002 -name=C -boot=127.0.0.1:7000 -encryptDHSOperations`



