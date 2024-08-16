{{- define "karmada.post-upgrade.configuration" -}}
---
apiVersion: config.karmada.io/v1alpha1
kind: ResourceInterpreterCustomization
metadata:
  name: karmada-preset-mutatingwebhookconfiguration
spec:
  target:
    apiVersion: admissionregistration.k8s.io/v1
    kind: MutatingWebhookConfiguration
  customizations:
    retention:
      luaScript: >
        function Retain(desiredObj, observedObj)
          local desiredLength = #desiredObj.webhooks
          local observedLength = #observedObj.webhooks
          if desiredLength <= 0 or observedLength <= 0 then
            return desiredObj
          end
          for i = 1, math.min(desiredLength, observedLength) do
             if desiredObj.webhooks[i].clientConfig.caBundle == nil then
               desiredObj.webhooks[i].clientConfig.caBundle = observedObj.webhooks[i].clientConfig.caBundle
             end
          end
          return desiredObj
        end
---
apiVersion: config.karmada.io/v1alpha1
kind: ResourceInterpreterCustomization
metadata:
  name: karmada-preset-deployment
spec:
  target:
    apiVersion: apps/v1
    kind: Deployment
  customizations:
    retention:
      luaScript: >
        function Retain(desiredObj, observedObj)
          -- 处理 restartAt annotation 支持 kubectl rollout restart 
          desiredObj.spec.template.metadata.annotations["kubectl.kubernetes.io/restartedAt"] = observedObj.spec.template.metadata.annotations["kubectl.kubernetes.io/restartedAt"]
          -- 处理 reloader environment variables
          desiredObj.spec.template.metadata.annotations["reloader.stakater.com/last-reloaded-from"] = observedObj.spec.template.metadata.annotations["reloader.stakater.com/last-reloaded-from"]
          if observedObj.spec.template.spec.containers ~= nil then
            for i, obsContainer in ipairs(observedObj.spec.template.spec.containers) do
              if obsContainer.env ~= nil then
                for _, env in ipairs(obsContainer.env) do
                  if string.find(env.name, "^STAKATER") then
                    -- 查找对应的 desiredObj container
                    if desiredObj.spec.template.spec.containers[i] ~= nil then
                      if desiredObj.spec.template.spec.containers[i].env == nil then
                        desiredObj.spec.template.spec.containers[i].env = {}
                      end
                      -- 添加或更新 env 变量
                      local found = false
                      for _, dEnv in ipairs(desiredObj.spec.template.spec.containers[i].env) do
                        if dEnv.name == env.name then
                          dEnv.value = env.value
                          found = true
                          break
                        end
                      end
                      if not found then
                        table.insert(desiredObj.spec.template.spec.containers[i].env, env)
                      end
                    end
                  end
                end
              end
            end
          end
          return desiredObj
        end
---
apiVersion: config.karmada.io/v1alpha1
kind: ResourceInterpreterCustomization
metadata:
  name: karmada-preset-validatingwebhookconfiguration
spec:
  target:
    apiVersion: admissionregistration.k8s.io/v1
    kind: ValidatingWebhookConfiguration
  customizations:
    retention:
      luaScript: >
        function Retain(desiredObj, observedObj)
          local desiredLength = #desiredObj.webhooks
          local observedLength = #observedObj.webhooks
          if desiredLength <= 0 or observedLength <= 0 then
            return desiredObj
          end
          for i = 1, math.min(desiredLength, observedLength) do
             if desiredObj.webhooks[i].clientConfig.caBundle == nil then
               desiredObj.webhooks[i].clientConfig.caBundle = observedObj.webhooks[i].clientConfig.caBundle
             end
          end
          return desiredObj
        end
{{- end -}}
