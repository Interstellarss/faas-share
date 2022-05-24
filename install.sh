echo Installing with helm 👑

helm repo add openfaas https://github.com/Interstellarss/faas-share

kubectl apply -f \
   https://raw.githubusercontent.com/Interstellarss/faas-share/master/namespaces.yml

# generate a random password
#PASSWORD=$(head -c 12 /dev/urandom | shasum| cut -d' ' -f1)

#kubectl -n openfaas create generic

echo "Installing chart 🍻"
helm upgrade \
    --install \
    openfaas \
    openfaas/faas-share\
    --namespace openfaas  \
    --set basic_auth=false \
    --set functionNamespace=faas-share-fn \
    --set serviceType=LoadBalancer \
    --wait
