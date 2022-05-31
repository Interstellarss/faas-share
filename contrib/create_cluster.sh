#!/usr/bin/env bash

DEVENV=${OF_DEV_ENV:-kind}
KUBE_VERSION=v1.21.12

EXISTS=$(kind get clusters | grep "$DEVENV")


curl -s https://packages.cloud.google.com/apt/doc/apt-key.gpg | sudo apt-key add - && \
  echo "deb http://apt.kubernetes.io/ kubernetes-xenial main" | sudo tee /etc/apt/sources.list.d/kubernetes.list && \
  sudo apt-get update -q && \
  sudo apt-get install -q --allow-downgrades kubelet=1.21.11-00 kubectl=1.21.11-00 kubeadm=1.21.11-00

	
sudo apt-get install -y kubernetes-cni


sudo swapoff -a

	
lsmod | grep br_netfilter
sudo modprobe br_netfilter

sudo sysctl net.bridge.bridge-nf-call-iptables=1


sudo mkdir /etc/docker
echo "{ "exec-opts": ["native.cgroupdriver=systemd"],
"log-driver": "json-file",
"log-opts":
{ "max-size": "100m" },
"storage-driver": "overlay2"
}" | sudo tee /etc/docker/daemon.json

sudo systemctl enable docker
sudo systemctl daemon-reload
sudo systemctl restart docker

sudo kubeadm reset
while true; do
read -p "Are you sure you want to proceed? [y/N]" yn
case $yn in
    [Yy]* ) break;;
    [Nn]* ) exit 1;;
    * ) echo "Please answer yes or no,";;
    esac
    

sudo kubeadm init --pod-network-cidr=10.244.0.0/16

mkdir -p $HOME/.kube
sudo cp -i /etc/kubernetes/admin.conf $HOME/.kube/config
sudo chown $(id -u):$(id -g) $HOME/.kube/config


sudo ufw allow 6443
sudo ufw allow 6443/tcp

kubectl apply -f https://raw.githubusercontent.com/coreos/flannel/master/Documentation/kube-flannel.yml
kubectl apply -f https://raw.githubusercontent.com/coreos/flannel/master/Documentation/k8s-manifests/kube-flannel-rbac.yml

kubectl get pods --all-namespaces

kubectl get cs

curl https://baltocdn.com/helm/signing.asc | gpg --dearmor | sudo tee /usr/share/keyrings/helm.gpg > /dev/null
sudo apt-get install apt-transport-https --yes
echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/helm.gpg] https://baltocdn.com/helm/stable/debian/ all main" | sudo tee /etc/apt/sources.list.d/helm-stable-debian.list
sudo apt-get update
sudo apt-get install helm

kubectl apply -f https://raw.githubusercontent.com/Interstellarss/faas-share/master/namespaces.yml
```

helm repo add faas-share  https://interstellarss.github.io/faas-share/

sudo helm repo update \
 && sudo helm upgrade faas-share --debug --install faas-share/faas-share \
    --namespace faas-share  \
    --set functionNamespace=faas-share-fn \
    --set generateBasicAuth=true

kubectl get pod -A

kubectl get event -n faas-share





###
#if [ "$EXISTS" = "$DEVENV" ]; then
#    while true; do
#    read -p "Kind cluster '$DEVENV' already exists, do you want to continue with this cluster? (yes/no) " yn
#    case $yn in
#        [Yy]* ) break;;
#        [Nn]* ) exit 1;;
#        * ) echo "Please answer yes or no.";;
#    esac
#done
#fi


#if [ "$EXISTS" != "$DEVENV" ]; then
#    echo ">>> Creating Kubernetes ${KUBE_VERSION} cluster ${DEVENV}"

#    kind create cluster --wait 5m --image kindest/node:${KUBE_VERSION} --name "$DEVENV" -v 1
#fi

#echo ">>> Waiting for CoreDNS"
#kubectl --context "kind-$DEVENV" -n kube-system rollout status deployment/coredns
