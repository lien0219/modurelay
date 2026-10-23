#!/usr/bin/env python3
"""Prepare a disposable Kubernetes stack for cluster-monitoring acceptance.

Requires Python 3 only. Generates secrets with mode 0600; never prints them.
The caller must use a dedicated kubeconfig and import the fixed images before
applying stack.json. This does not start or mutate Docker/user business data.
See cluster-monitoring-kubernetes-probe.py for live API verification.
"""
import json,secrets,os,pathlib
import argparse
parser=argparse.ArgumentParser(description="Generate an isolated Kubernetes monitoring acceptance stack. Apply only to an owned disposable cluster.")
parser.add_argument('--artifacts-dir',required=True)
parser.add_argument('--namespace',default='monitoring-acceptance')
parser.add_argument('--image',default='modurelay-monitoring-implementation:20261023')
parser.add_argument('--cluster-label',default='acceptance-k3s')
args=parser.parse_args()
p=pathlib.Path(args.artifacts_dir);p.mkdir(parents=True,exist_ok=True);ns=args.namespace
if not (ns.startswith('monitoring-') or ns.startswith('modurelay-monitoring-')):parser.error('disposable monitoring namespace required')
if (p/'credentials.json').exists():parser.error('artifacts directory already has credentials; use a fresh directory to avoid overwriting a running test')
s={'db_password':secrets.token_hex(16),'admin_password':secrets.token_hex(16),'metrics_token':secrets.token_hex(24),'jwt':secrets.token_hex(32)}
(p/'credentials.json').write_text(json.dumps(s));os.chmod(p/'credentials.json',0o600)
items=[]
def obj(kind,name,spec=None,**kw):
 d={'apiVersion':kw.pop('apiVersion','v1'),'kind':kind,'metadata':{'name':name,**({'namespace':ns} if kind not in ['Namespace','ClusterRole','ClusterRoleBinding'] else {})},**kw}
 if spec is not None:d['spec']=spec
 items.append(d);return d
def env(d):return [{'name':k,'value':v} for k,v in d.items()]
def dep(name,image,port,extra=None,environment=None,limits=None):
 c={'name':name,'image':image,'imagePullPolicy':'IfNotPresent','ports':[{'containerPort':port,'name':'http'}]}
 if extra:c.update(extra)
 if environment:c['env']=env(environment)
 if limits:c['resources']={'requests':{'cpu':'50m','memory':'64Mi'},'limits':limits}
 return obj('Deployment',name,{'replicas':1,'selector':{'matchLabels':{'app':name}},'template':{'metadata':{'labels':{'app':name}},'spec':{'serviceAccountName':'monitor','containers':[c]}}},apiVersion='apps/v1')
def svc(name,port,target=None):obj('Service',name,{'selector':{'app':target or name},'ports':[{'name':'metrics' if name=='modurelay-metrics' else 'http','port':port,'targetPort':port}]})
obj('Namespace',ns)
obj('ServiceAccount','monitor')
obj('ClusterRole','modurelay-monitoring-acceptance',apiVersion='rbac.authorization.k8s.io/v1',rules=[{'apiGroups':[''],'resources':['nodes','nodes/proxy','services','endpoints','pods','events','namespaces'],'verbs':['get','list','watch']},{'apiGroups':['apps'],'resources':['deployments','replicasets'],'verbs':['get','list','watch']},{'apiGroups':['autoscaling'],'resources':['horizontalpodautoscalers'],'verbs':['get','list','watch']}])
obj('ClusterRoleBinding','modurelay-monitoring-acceptance',apiVersion='rbac.authorization.k8s.io/v1',roleRef={'apiGroup':'rbac.authorization.k8s.io','kind':'ClusterRole','name':'modurelay-monitoring-acceptance'},subjects=[{'kind':'ServiceAccount','name':'monitor','namespace':ns}])
obj('Secret','credentials',type='Opaque',stringData={'db_password':s['db_password'],'admin_password':s['admin_password'],'metrics_token':s['metrics_token'],'jwt':s['jwt']})
pg=dep('postgres','postgres:18.1-alpine3.23',5432,environment={'POSTGRES_USER':'audit','POSTGRES_DB':'audit'},limits={'cpu':'500m','memory':'512Mi'})
pg['spec']['template']['spec']['containers'][0]['env'].append({'name':'POSTGRES_PASSWORD','valueFrom':{'secretKeyRef':{'name':'credentials','key':'db_password'}}})
svc('postgres',5432)
dep('redis','redis:7-alpine',6379,extra={'args':['redis-server','--save','','--appendonly','no']},limits={'cpu':'200m','memory':'128Mi'});svc('redis',6379)
app=dep('modurelay',args.image,8080,environment={'AUTO_SETUP':'true','SERVER_HOST':'0.0.0.0','SERVER_PORT':'8080','SERVER_MODE':'release','DATABASE_HOST':'postgres','DATABASE_PORT':'5432','DATABASE_USER':'audit','DATABASE_DBNAME':'audit','DATABASE_SSLMODE':'disable','DATABASE_MAX_OPEN_CONNS':'10','DATABASE_MAX_IDLE_CONNS':'2','REDIS_HOST':'redis','REDIS_PORT':'6379','REDIS_POOL_SIZE':'20','REDIS_MIN_IDLE_CONNS':'2','ADMIN_EMAIL':'audit@example.test','OPS_ENABLED':'true','OPS_METRICS_ENABLED':'true','OPS_MONITORING_MODE':'kubernetes','OPS_MONITORING_CLUSTER_NAME':args.cluster_label,'OPS_MONITORING_KUBERNETES_EVENTS_ENABLED':'true','OPS_MONITORING_KUBERNETES_RESOURCES_ENABLED':'true','OPS_MONITORING_EVENTS_NAMESPACE':ns,'OPS_MONITORING_PROMETHEUS_URL':'http://prometheus:9090','TZ':'UTC'},limits={'cpu':'500m','memory':'384Mi'})
a=app['spec']['template']['spec']; a['securityContext']={'fsGroup':1000};a['volumes']=[{'name':'data','emptyDir':{}}]
c=a['containers'][0];c['volumeMounts']=[{'name':'data','mountPath':'/app/data'}]
c['readinessProbe']={'httpGet':{'path':'/health','port':8080},'initialDelaySeconds':10,'periodSeconds':5}
for n,k in [('DATABASE_PASSWORD','db_password'),('ADMIN_PASSWORD','admin_password'),('OPS_METRICS_TOKEN','metrics_token'),('JWT_SECRET','jwt')]:c['env'].append({'name':n,'valueFrom':{'secretKeyRef':{'name':'credentials','key':k}}})
c['env'].append({'name':'POD_NAMESPACE','valueFrom':{'fieldRef':{'fieldPath':'metadata.namespace'}}})
svc('modurelay-metrics',8080,'modurelay')
ksm=dep('kube-state-metrics','registry.k8s.io/kube-state-metrics/kube-state-metrics:v2.20.0',8080,extra={'args':['--resources=pods,replicasets,deployments,horizontalpodautoscalers','--namespaces='+ns]},limits={'cpu':'200m','memory':'128Mi'});svc('kube-state-metrics',8080)
prom='''global:
  scrape_interval: 10s
  evaluation_interval: 10s
scrape_configs:
  - job_name: modurelay-cluster
    authorization: {type: Bearer, credentials_file: /secrets/metrics_token}
    static_configs:
      - targets: ['modurelay-metrics:8080']
        labels: {cluster: acceptance-k3s}
  - job_name: modurelay-instance
    metrics_path: /metrics/instance
    authorization: {type: Bearer, credentials_file: /secrets/metrics_token}
    kubernetes_sd_configs:
      - role: endpoints
        namespaces: {names: [monitoring-acceptance]}
    relabel_configs:
      - source_labels: [__meta_kubernetes_service_name, __meta_kubernetes_endpoint_port_name]
        action: keep
        regex: modurelay-metrics;metrics
      - target_label: cluster
        replacement: acceptance-k3s
      - source_labels: [__meta_kubernetes_namespace]
        target_label: namespace
      - source_labels: [__meta_kubernetes_service_name]
        target_label: service
      - source_labels: [__meta_kubernetes_pod_name]
        target_label: pod
      - source_labels: [__meta_kubernetes_pod_uid]
        target_label: uid
  - job_name: kube-state-metrics
    static_configs:
      - targets: ['kube-state-metrics:8080']
        labels: {cluster: acceptance-k3s}
  - job_name: cadvisor
    scheme: https
    authorization: {credentials_file: /var/run/secrets/kubernetes.io/serviceaccount/token}
    tls_config: {ca_file: /var/run/secrets/kubernetes.io/serviceaccount/ca.crt}
    kubernetes_sd_configs:
      - role: node
    relabel_configs:
      - target_label: cluster
        replacement: acceptance-k3s
      - target_label: __address__
        replacement: kubernetes.default.svc:443
      - source_labels: [__meta_kubernetes_node_name]
        target_label: __metrics_path__
        regex: (.+)
        replacement: /api/v1/nodes/$1/proxy/metrics/cadvisor
'''
prom=prom.replace('acceptance-k3s',args.cluster_label).replace('monitoring-acceptance',ns)
obj('ConfigMap','prometheus',data={'prometheus.yml':prom})
pr=dep('prometheus','prom/prometheus:v3.5.0',9090,extra={'args':['--config.file=/etc/prometheus/prometheus.yml','--storage.tsdb.path=/prometheus','--storage.tsdb.retention.time=2h','--storage.tsdb.retention.size=100MB']},limits={'cpu':'500m','memory':'384Mi'})
pp=pr['spec']['template']['spec'];pp['securityContext']={'fsGroup':65534};pp['volumes']=[{'name':'config','configMap':{'name':'prometheus'}},{'name':'secrets','secret':{'secretName':'credentials','items':[{'key':'metrics_token','path':'metrics_token'}]}},{'name':'data','emptyDir':{}}];pp['containers'][0]['volumeMounts']=[{'name':'config','mountPath':'/etc/prometheus'},{'name':'secrets','mountPath':'/secrets','readOnly':True},{'name':'data','mountPath':'/prometheus'}];svc('prometheus',9090)
(p/'stack.json').write_text(json.dumps({'apiVersion':'v1','kind':'List','items':items}));os.chmod(p/'stack.json',0o600)
print('Prepared dedicated stack resources:',len(items),'(credentials excluded from output)')
