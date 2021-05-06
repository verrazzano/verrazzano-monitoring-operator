# Usage

The document describes how to use the VMO in a standalone context, outside of the full Verrazzano context.

## Prerequisites

The following is required to run the operator:

* Kubernetes v1.13.x and up
* [Helm v2.9.1](https://github.com/kubernetes/helm/releases/tag/v2.9.1) and up

## Installation

### Install the CRDs required by VMO

```
kubectl apply -f k8s/crds/verrazzano-monitoring-operator-crds.yaml --validate=false
```

### Install Nginx Ingress Controller

```
helm upgrade ingress-controller stable/nginx-ingress --install --version 1.27.0  --set controller.service.enableHttp=false \
  --set controller.scope.enabled=true
```

### Install VMO

```
kubectl apply -f k8s/manifests/verrazzano-monitoring-operator.yaml
```

This will deploy the latest VMO image, or you can fill in a specific VMO image.

## VMI Examples

#### Simple VMI using NodePort access

To deploy a simple VMI:

Prepare a secret with the VMI username/password:
```
kubectl create secret generic vmi-secrets \
      --from-literal=username=vmo \
      --from-literal=password=changeme
```

Then:
```
kubectl apply -f k8s/examples/simple-vmi.yaml
```

Now, view the artifacts that the VMO created:

```
kubectl get deployments
NAME                                             DESIRED   CURRENT   UP-TO-DATE   AVAILABLE   AGE
vmi-vmi-1-api                                       1/1     1            0           35s
vmi-vmi-1-es-data-0                                 1/1     1            0           35s
vmi-vmi-1-es-exporter                               1/1     1            0           35s
vmi-vmi-1-es-ingest                                 1/1     1            0           35s
vmi-vmi-1-grafana                                   1/1     1            0           35s
vmi-vmi-1-kibana                                    1/1     1            0           35s
vmi-vmi-1-prometheus-0                              1/1     1            0           35s

kubectl get services
NAME                        TYPE        CLUSTER-IP      EXTERNAL-IP   PORT(S)          AGE
vmi-vmi-1-alertmanager           NodePort       10.96.120.46    <none>           9093:31685/TCP                        58s
vmi-vmi-1-alertmanager-cluster   ClusterIP      None            <none>           9094/TCP                              58s
vmi-vmi-1-api                    NodePort       10.96.83.126    <none>           9097:32645/TCP                        57s
vmi-vmi-1-es-data                NodePort       10.96.249.102   <none>           9100:32535/TCP                        58s
vmi-vmi-1-es-exporter            NodePort       10.96.95.21     <none>           9114:30699/TCP                        57s
vmi-vmi-1-es-ingest              NodePort       10.96.22.40     <none>           9200:30090/TCP                        58s
vmi-vmi-1-es-master              ClusterIP      None            <none>           9300/TCP                              58s
vmi-vmi-1-grafana                NodePort       10.96.125.142   <none>           3000:30634/TCP                        59s
vmi-vmi-1-kibana                 NodePort       10.96.142.26    <none>           5601:30604/TCP                        57s
vmi-vmi-1-prometheus             NodePort       10.96.187.224   <none>           9090:30053/TCP,9100:32382/TCP         59s
```

Now, access the endpoints for the various components, for example (based on the above output).  Note that this only works
on a Kubernetes cluster with worker nodes with public IP addresses.
* Grafana: http://worker_external_ip:30634
* Prometheus: http://worker_external_ip:30053
* Alertmanager: http://worker_external_ip:31685
* Kibana: http://worker_external_ip:30604
* Elasticsearch: http://worker_external_ip:30090

#### VMI with Data Volumes

This example specifies storage for the various VMO components, allowing VMI components' data to
survive across pod restarts and node failure.

```
kubectl apply -f k8s/examples/vmi-with-data-volumes.yaml
```

In addition to the artifacts created by the Simple VMI example, this also results in the creation of PVCs:

```
kubectl get pvc
NAME                         STATUS   VOLUME                                                                                      CAPACITY   ACCESS MODES   STORAGECLASS   AGE
vmi-vmi-1-es-data            Bound    ocid1.volume.oc1.uk-london-1.abwgiljrpmpozhpi554dcybqvwzjhxyje2pkhc74fiotuvdkids424ywne3a   50Gi       RWO            oci            30s
vmi-vmi-1-grafana            Bound    ocid1.volume.oc1.uk-london-1.abwgiljtupi46mdohk4hhnpy2laipwpfk3p44pizkrwdyft3p2vukkh2p2yq   50Gi       RWO            oci            30s
vmi-vmi-1-prometheus         Bound    ocid1.volume.oc1.uk-london-1.abwgiljtqe3v3zzyo7hwgeq4f3la5j44cxum6353rpzw55xocxvtaxuz5gqa   50Gi       RWO            oci            30s
```

#### VMI with Ingress, manaully created cert, no DNS

This examples requires that the ingress-controller you deployed above has succeeded in creating a LoadBalancer:

```
kubectl get svc
NAME                                               TYPE           CLUSTER-IP      EXTERNAL-IP      PORT(S)                               AGE
ingress-controller-nginx-ingress-controller        LoadBalancer   10.96.203.26    140.238.80.114   443:31379/TCP                         91s
```

Using an ingress-controller _without_ a separate cert-manager requires that we create a TLS secret manually for this VMI inage.  We'll 
create a self-signed cert for this example:

```
export DNSDOMAINNAME=dev.vmi1.verrazzano.io
# NOTE - double check your operating system's openssl.cnf location...
cp /etc/ssl/openssl.cnf /tmp/
echo '[ subject_alt_name ]' >> /tmp/openssl.cnf
echo "subjectAltName = DNS:*.$DNSDOMAINNAME, DNS:api.$DNSDOMAINNAME, DNS:grafana.$DNSDOMAINNAME, DNS:help.$DNSDOMAINNAME, DNS:kibana.$DNSDOMAINNAME, DNS:prometheus.$DNSDOMAINNAME, DNS:elasticsearch.$DNSDOMAINNAME" >> /tmp/openssl.cnf
openssl req -x509 -nodes -newkey rsa:2048 \
  -config /tmp/openssl.cnf \
  -extensions subject_alt_name \
  -keyout tls.key \
  -out tls.crt \
  -subj "/C=US/ST=Oregon/L=Portland/O=VMO/OU=PDX/CN=*.$DNSDOMAINNAME/emailAddress=postmaster@$DNSDOMAINNAME"
kubectl create secret tls vmi-1-tls --key=tls.key --cert=tls.crt
```

And create the VMI:
```
kubectl apply -f k8s/examples/vmi-with-ingress.yaml
```

Now, we can access our VMI endpoints through the LoadBalancer.  In the above example, our LoadBalancer IP is 140.238.80.114, and our VMI 
base URI is dev.vmi1.verrazzano.io.  We'll use host headers:

```
curl -k --user vmo:changeme https://140.238.80.114 --header "Host: grafana.dev.vmi1.verrazzano.io"
curl -k --user vmo:changeme https://140.238.80.114 --header "Host: prometheus.dev.vmi1.verrazzano.io"
curl -k --user vmo:changeme https://140.238.80.114 --header "Host: kibana.dev.vmi1.verrazzano.io"
curl -k --user vmo:changeme https://140.238.80.114 --header "Host: elasticsearch.dev.vmi1.verrazzano.io"
curl -k --user vmo:changeme https://140.238.80.114 --header "Host: api.dev.vmi1.verrazzano.io"
```

# VMI with Ingress, external-dns and cert-manager

The VMO was designed to work with [external-dns](https://github.com/helm/charts/tree/master/stable/external-dns) and 
[cert-manager](https://github.com/jetstack/cert-manager), and adds the appropriate ingress annotations to 
the created ingresses to trigger external-dns and cert-manager to take effect.

If cert-manager is installed via Helm prior to running the above example, the manual step to create the TLS cert isn't necessary.

Similarly, if external-dns is installed via Helm prior to running the above example, passing in host headers to our curl 
commands isn't necessary.
