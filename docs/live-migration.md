Live migration is the act of moving a virtual machine from one
hypervisor to another while maintaining the connectivity and liveness
of the applications running on the virtual machine.  It is one of the
primary advantages of using an external VM management service.


Triggering a live migration requires that the user perform an API
call.

To Create (start migration):
POST /apis/kubevirt.io/v1alpha1/namespaces/{namespace}/migrations 



Here is an example of a  migration request in YAML format:

```yaml

apiVersion: kubevirt.io/v1alpha1
kind: Migration
metadata:
  name: testvm-migration
spec:
  selector:
    name: testvm
```



The selector section indicates which vm objects are to be
migrated. The name field shown here would match a vm with the `name`
value of `testvm.`



return codes:

201 Created:  Does not indicate that the migration will be performed,
              but rather that the request has been accepted by the API
              server.

409 Conflict: if a migration with the same name is already present in
              the API server.


To list all migrations:

GET /apis/v1alpha1/namespaces/<namespace>/migrations/




To query a specific migration:

GET /apis/v1alpha1/namespaces/<namespace>/migrations/<migration_id>

return codes:

200 OK:  
404 Not Found:



Status field of the controller:

```yaml


apiVersion: kubevirt.io/v1alpha1
kind: Migration
metadata:
  creationTimestamp: 2017-03-15T11:25:47Z
  name: testvm-migration
  namespace: default
  resourceVersion: "96512"
  selfLink: /apis/kubevirt.io/v1alpha1/namespaces/default/migrations/testvm-migration
  uid: 230cb349-0972-11e7-b9cf-525400b9ab10
spec:
  selector:
    name: testvm
status:
  phase: Succeeded

```


Conditions can contain the following states: True, False and Unknown
with additional metadata like timestamps. See e.g.

Kubectl


To cancel:

DELETE /v1alpha1/namespaces/<namespace>/migrations/<migration_id>


Response codes:
 200 OK:
 404 Not Found:


 A Virtual machine is implemented as a Third Party Resource
 (TPR). Each successfully running virtual machine object has an
 associated Pod that contains the VM as a process. When a Pod is
 scheduled onto a node, it stays there until it is deleted. and
 initially has the same constraint. A Pod is completely immutable
 after creation. A VM is mutable regarding to scheduling (cluster)
 after creation, but those changes will not take effect until a
 migration takes place.  For example, to pin a VM to a specific node,
 the VM might be launched with the node selection criteria

```
nodeSelector: 
  kubernetes.io/hostname: node0
```

Which would pin it to node0.  In order to migrate to another node, the
user would first have to remove (via put or patch) the key and value
`kubernetes.io/hostname: node0`.  This *would not* force a
migration, it would only  *allow* a migration to take place.


A migration also has an associated pod that runs the migration
process. When the migration completes, the process running in the
pod's container exits.  The return code of the process indicates the
status of the migration.  This is translated into the status field of
the migration object.

Because the Kubernetes design calls for long polling the API server,
identifying changes, and asynchronous communication between
components, the flow is somewhat complex and spans multiple processes,
containers, and nodes. 

Participants:

Migration -- The Third Party Resource that represents a  migration
  Request created by User.

MigrationController -- Controller watching for migration
  requests that creates the VM target Pod
  
MigrationPodController -- Controller that Watches for state changes on
  the Pod which is speaking to libvirtd, triggering and monitoring the
  migration.

MigrationPod -- Pod which contains the process that runs the migration

VMSourcePod -- Pod which contains the virtual machine prior to the
  migration

VMDestinationPod -- Pod which contains the virtual machine after the
  migration

PodController  -- Controller's primary job is managing the Pods for
  Virtual machines. The PodController Participates in the migration
  process by identifying when the Target Pod is ready to recieve a
  migrating virtual machine, and starts the Pod that actually performs
  the migration.

VirtualMachine -- The Third Party Resource that represents a running
  virtual machine that the user wishes to migrate.


This is the sequence of a successful live migration.


1. Kubectl creates Migration TPR by speaking to the Kube API
  1. POST kubevirt.io/v1alpha1/namespaces/<namespace>/migrations/
1. MigrationController sees new Migration TPR
  1. Assuming that we are only dealing with “Make it happen now” migrations….
1. The MigrationController 
  1. creates an in memory plan from the unmarshalled Migration spec
  1. fetches the VMSourcePod spec from the VM Controller
  1. Merges the VMSourcePod spec data into the migration spec data to 
    1. Sets the source node on the plan from the VM spec
    1. Collision check the node selectors between the VM spec and the Migration spec to ensure we do not override the existing selectors.
    1. In order to ensure that the migration happens on a new node, add an anti affinity rule to the target Pod regarding to the Node where the source Pod is running on. 
  1. Create VMDestinationPod spec based on plan
  1. Post request for VMDestinationPod 
1. The MigrationController long polls to track the state of the newly launched pod
  1. If the Destination node == the Source node, set the migration status to Error
1. Once the newly scheduled VMDestinationPod has a virt-launcher process running
  (same as usual start flow), so the Pod is now RUNNING.
1. PodController identifies that a new VM Pod is running and Starts
  the MigrationPod
1. MigrationPodController identifies that the MigrationPod has been
started
  1.   
1. After MigrationController knows about a new destination node, it sets a migrationNodeName field on the VM spec
  1. Source virt handler will also see that  is set, and not recreate
  a VM that disappears, assuming it has migrated. 
1. Virt-handler is long polling for VMs filtered on both NodeName (for
existing on newly launched VMs) and migrationNodeName (for  VMs that
are about to migrate to the node)
1. Migration pod connects to the two libvirt instances on the nodes and triggers the migration (simplest one is really “virsh migrate …”)
  1. Error Handling: Source virt handler will also see that  migrationNodeName is set, and not recreate a VM that disappears, assuming it has migrated.  If there is an error in the flow, MigrationController removes the migrationNodeName, and the source virt-handler is responsible again for restarting, or, just set the node name to the target node and the target virt-handler will create the vm    
  1. If target  virt-handler sees the new VM and identifies that it does not have a spec yet for it, it should check the VM status and see that it is migrating, and not destroy it on sight.
  1. Race condition mitigation: If Virt-handler has not picked up
  the change to migrationNodeName during its long poll identifies
  that a new domain is created that it  does not know about.  It makes
  an explicit request to the kube-apiserver with filter which now
  picks up the new VM based on migrationNodeName or NodeName
  
1. MigrationPodController retries if necessary based on the specification and timeouts in mind 
1. MigrationPod completes successfully,
1. MigrationController updates the VM Spec with the new node
1. The MigrationController marks itself as in-sync with the situation.
1. virtHandler sees that VM’s NodeName is updated, and now assumes management of the VM


References

[1] https://kubernetes.io/docs/user-guide/rolling-updates/





