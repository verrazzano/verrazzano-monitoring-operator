# Verrazzano Integration

In the full Verrazzano context, the VMO is installed by the [Verrazzano installer](https://github.com/verrazzano/verrazzano)
into a Verrazzano "Management Cluster".  The Verrazzano installer picks up the VMO through the verrazzano-helm-chart.
From there, users create VerrazzanoModels and VerrazzanoBindings, describing their applications.  When a VerrazzanoBinding is 
created, the [verrazzano-operator](https://github.com/verrazzano/verrazzano-operator) notices this and creates a 
VMI in the Management Cluster, which is intended to collect metrics and logs about the application and system.  From there, the 
VMO is responsible for managing the lifecycle of those VMI.
