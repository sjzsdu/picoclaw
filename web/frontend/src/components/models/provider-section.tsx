import { IconChevronDown, IconLoader2 } from "@tabler/icons-react"
import { useState } from "react"
import { useTranslation } from "react-i18next"

import type { ModelInfo } from "@/api/models"
import { Button } from "@/components/ui/button"

import { ModelCard } from "./model-card"
import { ProviderIcon } from "./provider-icon"

interface ProviderSectionProps {
  provider: string
  providerKey: string
  models: ModelInfo[]
  onEdit: (model: ModelInfo) => void
  onTest: (model: ModelInfo) => void
  onTestAll: (providerKey: string) => void
  onSetDefault: (model: ModelInfo) => void
  onDelete: (model: ModelInfo) => void
  testingIndex: number | null
  testingAll: boolean
  settingDefaultIndex: number | null
}

export function ProviderSection({
  provider,
  providerKey,
  models,
  onEdit,
  onTest,
  onTestAll,
  onSetDefault,
  onDelete,
  testingIndex,
  testingAll,
  settingDefaultIndex,
}: ProviderSectionProps) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(true)

  return (
    <section className="my-8">
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        className="mb-3 grid w-full grid-cols-[1fr_auto_1fr_auto] items-center gap-2 px-1 py-1.5 text-left"
        aria-expanded={open}
      >
        <div className="border-border/40 border-t" />
        <span className="text-foreground/80 text-center text-xs font-semibold tracking-wide uppercase">
          <span className="bg-background inline-flex items-center gap-1.5 px-2">
            <ProviderIcon providerKey={providerKey} providerLabel={provider} />
            {provider}
          </span>
        </span>
        <div className="border-border/40 border-t" />
        <span className="flex items-center justify-end gap-1">
          <Button
            size="sm"
            variant="ghost"
            onClick={(event) => {
              event.stopPropagation()
              onTestAll(providerKey)
            }}
            disabled={testingAll}
          >
            {testingAll ? (
              <IconLoader2 className="size-4 animate-spin" />
            ) : null}
            {t("models.action.testAll")}
          </Button>
          <IconChevronDown
            className={[
              "text-muted-foreground size-4 transition-transform",
              open ? "rotate-180" : "",
            ].join(" ")}
          />
        </span>
      </button>

      {open && (
        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
          {models.map((model) => (
            <ModelCard
              key={model.index}
              model={model}
              onEdit={onEdit}
              onTest={onTest}
              onSetDefault={onSetDefault}
              onDelete={onDelete}
              testing={testingIndex === model.index}
              testingAll={testingAll}
              settingDefault={settingDefaultIndex === model.index}
            />
          ))}
        </div>
      )}
    </section>
  )
}
