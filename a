apiVersion: admissionregistration.k8s.io/v1beta1
kind: ValidatingWebhookConfiguration
metadata:
  creationTimestamp: 2019-11-18T16:39:54Z
  generation: 2
  name: cdi-api-datavolume-validate
  ownerReferences:
  - apiVersion: cdi.kubevirt.io/v1alpha1
    blockOwnerDeletion: true
    controller: true
    kind: CDI
    name: cdi
    uid: c4a4cc4c-0a1c-11ea-9467-525500d15501
  resourceVersion: "13878"
  selfLink: /apis/admissionregistration.k8s.io/v1beta1/validatingwebhookconfigurations/cdi-api-datavolume-validate
  uid: 0ca15dd2-0a22-11ea-9467-525500d15501
webhooks:
- clientConfig:
    caBundle: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUMyakNDQWNLZ0F3SUJBZ0lCQURBTkJna3Foa2lHOXcwQkFRc0ZBREFlTVJ3d0dnWURWUVFERXhOaGNHa3UKWTJScExtdDFZbVYyYVhKMExtbHZNQjRYRFRFNU1URXhPREUyTkRFeE1sb1hEVEk1TVRFeE5URTJOREV4TWxvdwpIakVjTUJvR0ExVUVBeE1UWVhCcExtTmthUzVyZFdKbGRtbHlkQzVwYnpDQ0FTSXdEUVlKS29aSWh2Y05BUUVCCkJRQURnZ0VQQURDQ0FRb0NnZ0VCQUxZSHU0VUIxTWMyalhZc2lZMVF3aXZGYUc2bVpQZkxhUm1lcE9FTUxudHUKZ1ZQUnU2UVV2TFNSRktVaTQrenIxbGpIMGdza1N0NHJuQWxZbUdqOTFtQmpEakxFdUZRcXpNRlN0VFkwVWpPMgpmK2M3U0hRUzVZaVlSNTFQZXVWblRWZnUzcWhzdlpIbytveTNiN2pQQ1A3aXNrazRDRnR1SURKb1lFTGZ5YnNvCnNOT1JXdzUzNElWdTFwdHRFVEh3OFkxWVBNNFROVm9UVi82V2w0bTB6MG5MZitteXNGTlN5ZzlUUFJDRXBhcWYKRk5oYnpxVzIvZFF2a1YyVk82ZnFqREo0VzZjYmRYMmZHbW5aUkhIQzhsWGVzOWFiOFpMOUxJdVdNSGhXUDRNTwprTE40R3hOazBzbm9keXR4ZUViUGhLKzJDaDN6czlMMUo2Y3hhUEc4alRNQ0F3RUFBYU1qTUNFd0RnWURWUjBQCkFRSC9CQVFEQWdLa01BOEdBMVVkRXdFQi93UUZNQU1CQWY4d0RRWUpLb1pJaHZjTkFRRUxCUUFEZ2dFQkFLNGgKVXFIZjZnVHlCVVI2ZlNXUUxqSnpoRXdTMnRKekJkU3RWand3MmdMeHVEbzZ1QnlRTVlKR244eGhkRFdpeFdQdQo5VmxybGFRcWpEYUVTUWJnTGxGUmZxSnBNQXZ4b2R6ZmZXTW1qRTQwL0VIMUpkSUNCVSt4QjBvd2MvRVBSN1dLCmF1V3hERUJhUU1QcnFVQnZYVnpTRjh4RWF4Sm82dzB1OVdsWlRMN2s0Vm9qWFFLN1lja1Q4VStMQmRXdzVNSjUKMDFPVTZpcHNKN05TUmtZMTRXMFlVNkxVa1pkRGJXMEtLSkZReTkxMlNXMzRPNXM1eUkxV2dENldmbXhPdUpYWApLQk9FK0FWd0NyRVVRa3dCeThjMjRQVThnRG5QZXorRkh2WU0vNzZ0My9GWmVuZFBCM3RpM3l4bXBaQTZjcGkyCmpJc0VoakFhWCtIOTFaWEtGTTA9Ci0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0K
    service:
      name: cdi-api
      namespace: cdi
      path: /datavolume-validate
  failurePolicy: Fail
  name: datavolume-validate.cdi.kubevirt.io
  namespaceSelector: {}
  rules:
  - apiGroups:
    - cdi.kubevirt.io
    apiVersions:
    - v1alpha1
    operations:
    - CREATE
    - UPDATE
    resources:
    - datavolumes
