import {
  DndContext,
  DragOverlay,
  PointerSensor,
  closestCenter,
  useSensor,
  useSensors,
  type DragEndEvent,
  type DragStartEvent,
} from "@dnd-kit/core"
import {
  SortableContext,
  useSortable,
  horizontalListSortingStrategy,
} from "@dnd-kit/sortable"
import { CSS } from "@dnd-kit/utilities"
import { IconGripVertical, IconPlus, IconX } from "@tabler/icons-react"
import { useCallback, useEffect, useMemo, useRef, useState } from "react"
import { useTranslation } from "react-i18next"

import { cn } from "@/lib/utils"
import type { TabState } from "@/store/chat"

interface ChatTabBarProps {
  tabs: TabState[]
  activeTabIndex: number
  onSelectTab: (index: number) => void
  onCloseTab: (index: number) => void
  onNewTab: () => void
  onReorderTabs: (fromIndex: number, toIndex: number) => void
  maxTabsReached?: boolean
}

function tabLabel(tab: TabState, index: number, t: ReturnType<typeof useTranslation>["t"]): string {
  if (tab.title) return tab.title
  const firstUserMsg = tab.messages.find((m) => m.role === "user")
  if (firstUserMsg) {
    const text = firstUserMsg.content.trim()
    if (text) return text.length > 20 ? `${text.slice(0, 20)}...` : text
  }
  return `${t("chat.newChat")} ${index + 1}`
}

interface SortableTabProps {
  tab: TabState
  isActive: boolean
  label: string
  tabsLength: number
  onSelect: () => void
  onClose: (e: React.MouseEvent) => void
  onKeyDownClose: (e: React.KeyboardEvent) => void
  t: ReturnType<typeof useTranslation>["t"]
}

function SortableTab({
  tab,
  isActive,
  label,
  tabsLength,
  onSelect,
  onClose,
  onKeyDownClose,
  t,
}: SortableTabProps) {
  const {
    attributes,
    listeners,
    setNodeRef,
    transform,
    transition,
    isDragging,
  } = useSortable({ id: tab.sessionId })

  const style: React.CSSProperties = {
    transform: isDragging ? undefined : CSS.Transform.toString(transform),
    transition,
    opacity: isDragging ? 0 : 1,
  }

  return (
    <button
      ref={setNodeRef}
      type="button"
      style={style}
      onClick={onSelect}
      data-active={isActive ? "true" : undefined}
      className={cn(
        "group relative flex max-w-48 shrink-0 cursor-grab items-center gap-1 rounded-t-md px-2.5 py-1.5 text-xs font-medium transition-[color,background,box-shadow] active:cursor-grabbing",
        isActive
          ? "bg-background text-foreground shadow-[0_1px_0_0_hsl(var(--background))] after:absolute after:inset-x-0 after:bottom-0 after:h-0.5 after:bg-foreground"
          : "text-muted-foreground hover:bg-muted/50 hover:text-muted-foreground/80",
        isDragging && "z-50 shadow-md ring-1 ring-border/50",
      )}
      {...attributes}
      {...listeners}
    >
      <IconGripVertical
        className="text-muted-foreground/30 shrink-0 opacity-0 transition-opacity group-hover:opacity-100"
        size={12}
      />
      <span className="truncate">{label}</span>
      {tabsLength > 1 && (
        <span
          role="button"
          tabIndex={0}
          className={cn(
            "ml-0.5 inline-flex size-4 shrink-0 items-center justify-center rounded-sm transition-colors",
            isActive ? "text-muted-foreground" : "text-muted-foreground/40",
            "hover:bg-destructive/15 hover:text-destructive group-hover:text-muted-foreground",
          )}
          onClick={onClose}
          onKeyDown={onKeyDownClose}
          aria-label={t("chat.closeTab")}
        >
          <IconX className="size-3" />
        </span>
      )}
    </button>
  )
}

function TabDragOverlay({ label }: { label: string }) {
  return (
    <div className="bg-background text-foreground flex max-w-48 items-center gap-1 rounded-t-md px-2.5 py-1.5 text-xs font-medium shadow-lg ring-1 ring-border/50">
      <IconGripVertical className="text-muted-foreground/30 size-3 shrink-0" />
      <span className="truncate">{label}</span>
    </div>
  )
}

export function ChatTabBar({
  tabs,
  activeTabIndex,
  onSelectTab,
  onCloseTab,
  onNewTab,
  onReorderTabs,
  maxTabsReached,
}: ChatTabBarProps) {
  const { t } = useTranslation()
  const scrollContainerRef = useRef<HTMLDivElement>(null)
  const [activeDragId, setActiveDragId] = useState<string | null>(null)

  const sensors = useSensors(
    useSensor(PointerSensor, {
      activationConstraint: { distance: 6 },
    }),
  )

  // Auto-scroll active tab into view
  useEffect(() => {
    const el = scrollContainerRef.current?.querySelector<HTMLElement>(
      '[data-active="true"]',
    )
    el?.scrollIntoView({
      behavior: "smooth",
      block: "nearest",
      inline: "nearest",
    })
  }, [activeTabIndex])

  const sessionIds = useMemo(() => tabs.map((t) => t.sessionId), [tabs])

  const handleDragStart = useCallback((event: DragStartEvent) => {
    setActiveDragId(String(event.active.id))
  }, [])

  const handleDragEnd = useCallback(
    (event: DragEndEvent) => {
      setActiveDragId(null)
      const { active, over } = event
      if (!over || active.id === over.id) return

      const oldIndex = tabs.findIndex((t) => t.sessionId === active.id)
      const newIndex = tabs.findIndex((t) => t.sessionId === over.id)
      if (oldIndex !== -1 && newIndex !== -1) {
        onReorderTabs(oldIndex, newIndex)
      }
    },
    [tabs, onReorderTabs],
  )

  const handleDragCancel = useCallback(() => {
    setActiveDragId(null)
  }, [])

  const handleClose = useCallback(
    (e: React.MouseEvent, index: number) => {
      e.stopPropagation()
      onCloseTab(index)
    },
    [onCloseTab],
  )

  const draggedTab = useMemo(
    () => (activeDragId ? tabs.find((t) => t.sessionId === activeDragId) : null),
    [activeDragId, tabs],
  )

  return (
    <DndContext
      sensors={sensors}
      collisionDetection={closestCenter}
      onDragStart={handleDragStart}
      onDragEnd={handleDragEnd}
      onDragCancel={handleDragCancel}
    >
      <div className="border-border/40 flex items-center border-b bg-[hsl(var(--background)_/_0.6)] pl-2 pr-1 backdrop-blur-sm">
        <div
          ref={scrollContainerRef}
          className="flex flex-1 overflow-x-auto [scrollbar-width:thin]"
        >
          <SortableContext
            items={sessionIds}
            strategy={horizontalListSortingStrategy}
          >
            <div className="flex items-end gap-0.5 py-1">
              {tabs.map((tab, index) => {
                const isActive = index === activeTabIndex
                const label = tabLabel(tab, index, t)
                return (
                  <SortableTab
                    key={tab.sessionId}
                    tab={tab}
                    isActive={isActive}
                    label={label}
                    tabsLength={tabs.length}
                    onSelect={() => onSelectTab(index)}
                    onClose={(e) => handleClose(e, index)}
                    onKeyDownClose={(e) => {
                      if (e.key === "Enter" || e.key === " ") {
                        e.preventDefault()
                        onCloseTab(index)
                      }
                    }}
                    t={t}
                  />
                )
              })}
            </div>
          </SortableContext>
        </div>

        <button
          type="button"
          onClick={onNewTab}
          disabled={maxTabsReached}
          className={cn(
            "text-muted-foreground mb-1 ml-1 inline-flex size-6 shrink-0 items-center justify-center rounded-md transition-colors",
            maxTabsReached
              ? "cursor-not-allowed opacity-30"
              : "hover:text-foreground hover:bg-muted",
          )}
          aria-label={t("chat.newTab")}
          title={
            maxTabsReached
              ? t("chat.tabsLimitReached", { max: tabs.length })
              : t("chat.newTab")
          }
        >
          <IconPlus className="size-4" />
        </button>
      </div>

      <DragOverlay>
        {draggedTab ? (
          <TabDragOverlay
            label={tabLabel(
              draggedTab,
              tabs.findIndex((t) => t.sessionId === draggedTab.sessionId),
              t,
            )}
          />
        ) : null}
      </DragOverlay>
    </DndContext>
  )
}
