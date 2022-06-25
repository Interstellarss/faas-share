
### Intall

```shell
kubectl apply -f deploy/crd.yaml 

kubectl apply -f https://raw.githubusercontent.com/Interstellarss/faas-share/master/namespaces.yml

helm repo add faas-share  https://interstellarss.github.io/faas-share/

helm repo update \
 && helm upgrade faas-share --debug --install faas-share/faas-share \
    --namespace faas-share  \
    --set functionNamespace=faas-share-fn \
    --set generateBasicAuth=false
```

### Examples:

checkout Action folder for Mobilenet, shufflenet, and squeezenet