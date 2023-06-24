# NoNat

NoNat is a CNI plugin that is attempting to rid Kubernetes of NAT (Network Address Translation). While IPv4 is coupled with NAT, IPV6 can get rid of NAT completely. Pod->Service traffic NAT can get completely removed. Let's build a CNI plugin and Kube-Proxy like agent to do what is needed to rid ourselves of NAT!

Version 1 Goals:
Remove DNAT for Pod -> Service traffic. Instead we are going to assign the ClusterIP address inside the pod network namespace and use anycast ecmp routing. 
