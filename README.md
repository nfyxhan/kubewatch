A tools to watch you kubernetes resource change.

# install 

```
go install github.com/nfyxhan/kubewatch@latest
```

# useage

kubewatch deployments status:

```
kubewatch watch --group-version apps/v1 deploy --path-prefix=Object,status
```

kubewatch some pods by name:
```
kubewatch watch --group-version v1 po -n default podinfo-745bb5b648-8w5lf podinfo-66975d5b8c-nkvpm 
```
