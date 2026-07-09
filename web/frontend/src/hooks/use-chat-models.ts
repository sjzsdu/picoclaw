import { useCallback, useEffect, useMemo, useState } from "react"
import { toast } from "sonner"

import { type ModelInfo, getModels } from "@/api/models"

interface UseChatModelsOptions {
  isConnected: boolean
  activeSessionId?: string
}

function isLocalModel(model: ModelInfo): boolean {
  const isLocalHostBase = Boolean(
    model.api_base?.includes("localhost") ||
    model.api_base?.includes("127.0.0.1"),
  )

  return (
    model.auth_method === "local" || (!model.auth_method && isLocalHostBase)
  )
}

export function useChatModels({
  isConnected,
  activeSessionId,
}: UseChatModelsOptions) {
  const [modelList, setModelList] = useState<ModelInfo[]>([])
  const [defaultModelName, setDefaultModelName] = useState("")
  const [sessionModelNames, setSessionModelNames] = useState<Record<string, string>>(
    {},
  )
  const [loadError, setLoadError] = useState<string | null>(null)

  const sessionModelName = activeSessionId
    ? (sessionModelNames[activeSessionId] ?? "")
    : ""

  const selectedModelName = useMemo(() => {
    const preferredModelName = sessionModelName || defaultModelName
    if (modelList.some((model) => model.model_name === preferredModelName)) {
      return preferredModelName
    }
    return ""
  }, [defaultModelName, modelList, sessionModelName])

  const syncDefaultModelName = useCallback(
    (models: ModelInfo[], defaultModel: string) => {
      if (models.some((m) => m.model_name === defaultModel)) {
        setDefaultModelName(defaultModel)
        return
      }
      setDefaultModelName("")
    },
    [],
  )

  const loadModels = useCallback(async () => {
    try {
      setLoadError(null)
      const data = await getModels()
      setModelList(data.models)
      syncDefaultModelName(data.models, data.default_model)
    } catch (err) {
      const msg = err instanceof Error ? err.message : "Unknown error"
      console.error("Failed to load models:", msg)
      setLoadError(msg)
      toast.error(
        "Failed to load models. Please check your connection and refresh.",
      )
    }
  }, [syncDefaultModelName])

  useEffect(() => {
    const timerId = setTimeout(() => {
      void loadModels()
    }, 0)

    return () => clearTimeout(timerId)
  }, [isConnected, activeSessionId, loadModels])

  const handleSelectModel = useCallback(
    (modelName: string) => {
      if (!activeSessionId || modelName === selectedModelName) return
      setSessionModelNames((prev) => ({
        ...prev,
        [activeSessionId]: modelName,
      }))
    },
    [activeSessionId, selectedModelName],
  )

  const defaultSelectableModels = useMemo(
    () =>
      modelList.filter(
        (m) => m.default_model_allowed !== false && m.is_virtual !== true,
      ),
    [modelList],
  )

  const hasAvailableModels = useMemo(
    () => defaultSelectableModels.some((m) => m.available),
    [defaultSelectableModels],
  )

  const oauthModels = useMemo(
    () =>
      defaultSelectableModels.filter(
        (m) => m.available && m.auth_method === "oauth",
      ),
    [defaultSelectableModels],
  )

  const localModels = useMemo(
    () => defaultSelectableModels.filter((m) => m.available && isLocalModel(m)),
    [defaultSelectableModels],
  )

  const apiKeyModels = useMemo(
    () =>
      defaultSelectableModels.filter(
        (m) => m.available && m.auth_method !== "oauth" && !isLocalModel(m),
      ),
    [defaultSelectableModels],
  )

  return {
    defaultModelName,
    selectedModelName,
    hasAvailableModels,
    apiKeyModels,
    oauthModels,
    localModels,
    handleSelectModel,
    loadError,
  }
}
