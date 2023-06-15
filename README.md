# NoNat

NoNat is a CNI plugin that is attempign to rid Kubernetes of NAT (Network Address Translation). While IPv4 is coupled with NAT, IPV6 can get rid of NAT completely. Pod->Service traffic NAT can get completely removed. Let's build a CNI plugin and Kube-Proxy like agent to do what is needed to rid ourselves of NAT!
