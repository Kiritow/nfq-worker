# nfq-worker

Better SNAT/DNAT/NETMAP tools

### What is it used for

A possible solution to overlapping subnet across the whole network. It can truly mapping one subnet to a not-overlapping subnet.

1. Change **destination ip** after final routing decision (`POSTROUTING`)

2. Change **source ip** before first routing decision (and conntrack) (`PREROUTING`)
 
### Why

1. iptables can't change destination ip in `POSTROUTING` chain.

2. iptables can't change source ip in `PREROUTING` chain.

3. `NETMAP` target is not worked as expected.

4. No other chains allow `SNAT`/`DNAT`/`NETMAP` targets.

### Usage

#### EGRESS

`nfq-worker --mode 1 --num 1 --from 10.0.0.0/24 --to 10.0.1.0/24`

`ip route add 10.0.1.0/24 dev <nic_out>`

`iptables -t mangle -A POSTROUTING -o <nic_out> -d <subnet> -j NFQUEUE --queue-num 1`

#### INGRESS

`nfq-worker --mode 2 --num 2 --from 10.0.1.0/24 --to 10.0.0.0/24`

`iptables -t raw -A PREROUTING -i <nic_in> -s <subnet> -j NFQUEUE --queue-num 2`
