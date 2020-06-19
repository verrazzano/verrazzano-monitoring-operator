# Verrazzano Integration

In the full Verrazzano context, the VMO is installed by the [verrazzano-installer](https://github.com/oracle/verrazzano-installer)
into a Verrazzano "Management Cluster".  The verrazzano-installer picks up the VMO through the verrazzano-helm-chart.
From there, users create VerrazzanoModels and VerrazzanoBindings, describing their applications.  When a VerrazzanoBinding is 
created, the [verrazzano-operator](https://github.com/oracle/verrazzano-operator) notices this and creates a 
VMI in the Management Cluster, which is intended to collect metrics and logs about the application and system.  From there, the 
VMO is responsible for managing the lifecycle of those VMI.
