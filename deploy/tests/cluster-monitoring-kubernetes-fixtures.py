#!/usr/bin/env python3
import json,pathlib
import argparse
p=argparse.ArgumentParser(description='Generate disposable Pod lifecycle and HPA fixtures');p.add_argument('--artifacts-dir',required=True);p.add_argument('--namespace',default='monitoring-acceptance');a=p.parse_args();ns=a.namespace;items=[]
if not ns.startswith(('monitoring-','modurelay-monitoring-')):p.error('disposable monitoring namespace required')
def pod(name,limits=None,init=None,resources=None,runtime=None,command=None,volumes=None):
 c={'name':'main','image':'redis:7-alpine','imagePullPolicy':'IfNotPresent','command':command or ['/bin/sh','-c','sleep 36000']}
 if limits:c['resources']={'requests':{'cpu':'10m','memory':'16Mi'},'limits':limits}
 s={'containers':[c]}
 if init:s['initContainers']=init
 if resources:s['resources']=resources
 if runtime:s['runtimeClassName']=runtime
 if volumes:s['volumes']=volumes;c['volumeMounts']=[{'name':'control','mountPath':'/control','readOnly':True}]
 items.append({'apiVersion':'v1','kind':'Pod','metadata':{'namespace':ns,'name':name},'spec':s})
def init(name,cpu,mem,restart=False):
 d={'name':name,'image':'redis:7-alpine','imagePullPolicy':'IfNotPresent','command':['/bin/sh','-c','sleep 36000' if restart else 'true'],'resources':{'requests':{'cpu':'10m','memory':'16Mi'},'limits':{'cpu':cpu,'memory':mem}}}
 if restart:d['restartPolicy']='Always'
 return d
pod('unlimited')
pod('ordinary',{'cpu':'250m','memory':'64Mi'})
pod('init-sidecar',{'cpu':'200m','memory':'64Mi'},[init('sidecar','100m','32Mi',True),init('prepare','500m','96Mi')])
pod('pod-level',resources={'limits':{'cpu':'1200m','memory':'256Mi'},'requests':{'cpu':'50m','memory':'32Mi'}})
items.append({'apiVersion':'node.k8s.io/v1','kind':'RuntimeClass','metadata':{'name':'monitoring-overhead'},'handler':'runc','overhead':{'podFixed':{'cpu':'50m','memory':'16Mi'}}})
pod('with-overhead',{'cpu':'200m','memory':'64Mi'},[init('prepare','500m','96Mi')],runtime='monitoring-overhead')
items.append({'apiVersion':'v1','kind':'ConfigMap','metadata':{'namespace':ns,'name':'crash-control'},'data':{'initial':'crash'}})
pod('crash-recovery',{'cpu':'100m','memory':'32Mi'},command=['/bin/sh','-c','if [ -f /control/healthy ]; then exec sleep 36000; fi; exit 1'],volumes=[{'name':'control','configMap':{'name':'crash-control'}}])
pod('oom-restart',{'cpu':'100m','memory':'64Mi'},command=['/bin/sh','-c',"sleep 75; exec awk 'BEGIN {s=\"x\"; for(i=0;i<30;i++)s=s s; print length(s)}'"])
items.append({'apiVersion':'autoscaling/v2','kind':'HorizontalPodAutoscaler','metadata':{'namespace':ns,'name':'modurelay'},'spec':{'scaleTargetRef':{'apiVersion':'apps/v1','kind':'Deployment','name':'modurelay'},'minReplicas':1,'maxReplicas':3,'metrics':[{'type':'Resource','resource':{'name':'cpu','target':{'type':'Utilization','averageUtilization':50}}}]}})
pathlib.Path(a.artifacts_dir,'fixtures.json').write_text(json.dumps({'apiVersion':'v1','kind':'List','items':items}));print('Prepared Pod lifecycle fixtures:',len(items))
