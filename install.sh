echo Installing with helm 👑

helm repo add faas-share https://github.com/Interstellarss/faas-share

kubectl apply -f \
   https://raw.githubusercontent.com/openfaas/faas-netes/master/namespaces.yml

# generate a random password
#PASSWORD=$(head -c 12 /dev/urandom | shasum| cut -d' ' -f1)

#kubectl -n openfaas create generic

echo "Installing chart 🍻"
helm upgrade \
    --install \
    faas-share \
    openfaas/openfaas \
    --namespace openfaas  \
    --set basic_auth=false \
    --set functionNamespace=faas-share-fn \
    --set serviceType=LoadBalancer \
    --wait
