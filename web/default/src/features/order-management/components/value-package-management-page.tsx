/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { useEffect, useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { RefreshCw, RotateCcw, Search, Sparkles } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { formatTimestampToDate } from '@/lib/format'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty'
import { Field, FieldGroup, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import {
  Pagination,
  PaginationContent,
  PaginationItem,
  PaginationNext,
  PaginationPrevious,
} from '@/components/ui/pagination'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Skeleton } from '@/components/ui/skeleton'
import { Spinner } from '@/components/ui/spinner'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Textarea } from '@/components/ui/textarea'
import { SectionPageLayout } from '@/components/layout'
import {
  getAdminPlans,
  patchPlanGiftResetCount,
} from '@/features/subscriptions/api'
import { ValuePackagePeriodList } from '@/features/value-packages/components/value-package-period-list'
import { getValuePackagePeriodLimits } from '@/features/value-packages/lib/period-limits'
import {
  adjustOrderManagementValuePackageResetCount,
  getOrderManagementValuePackageUsers,
  type GetOrderManagementValuePackageUsersParams,
} from '../api'
import type {
  OrderManagementValuePackageManagementRow,
  OrderManagementValuePackageResetCountAdjustMode,
} from '../types'

const PAGE_SIZE = 20

type PackageFilter = 'all' | 'day' | 'week' | 'month'
type ActiveFilter = 'active' | 'expired' | 'all'

const orderManagementValuePackageManagementKeys = {
  users: (params: GetOrderManagementValuePackageUsersParams) =>
    ['order-management', 'value-package-management', params] as const,
}

function packageLabel(packageType: string, t: (key: string) => string) {
  switch (packageType) {
    case 'day':
      return t('Day package')
    case 'week':
      return t('Week package')
    case 'month':
      return t('Month package')
    default:
      return t('Value Package')
  }
}

function formatResetTime(timestamp: number, t: (key: string) => string) {
  return timestamp > 0 ? formatTimestampToDate(timestamp) : t('Never')
}

function LoadingRows() {
  return Array.from({ length: 8 }, (_, index) => (
    <TableRow key={`value-package-management-skeleton-${index}`}>
      {Array.from({ length: 8 }, (_, cellIndex) => (
        <TableCell key={cellIndex}>
          <Skeleton className='h-5 w-24' />
        </TableCell>
      ))}
    </TableRow>
  ))
}

function ResetCountDialog({
  row,
  open,
  submitting,
  onOpenChange,
  onSubmit,
}: {
  row: OrderManagementValuePackageManagementRow | null
  open: boolean
  submitting: boolean
  onOpenChange: (open: boolean) => void
  onSubmit: (request: {
    userId: number
    mode: OrderManagementValuePackageResetCountAdjustMode
    value: number
    reason: string
  }) => void
}) {
  const { t } = useTranslation()
  const [mode, setMode] =
    useState<OrderManagementValuePackageResetCountAdjustMode>('set')
  const [value, setValue] = useState(String(row?.reset_count ?? 0))
  const [reason, setReason] = useState('')

  useEffect(() => {
    if (!open) return
    setMode('set')
    setValue(String(row?.reset_count ?? 0))
    setReason('')
  }, [open, row?.user_id, row?.reset_count])

  const submit = () => {
    if (!row) return
    const parsed = Number(value)
    if (!Number.isFinite(parsed) || parsed < 0) {
      toast.error(t('Reset count must be a non-negative number'))
      return
    }
    onSubmit({
      userId: row.user_id,
      mode,
      value: Math.floor(parsed),
      reason: reason.trim(),
    })
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t('Adjust reset count')}</DialogTitle>
          <DialogDescription>
            {row
              ? `${row.username || `#${row.user_id}`} · ${t('Current reset count')}: ${row.reset_count}`
              : t('Adjust reset count')}
          </DialogDescription>
        </DialogHeader>
        <FieldGroup>
          <Field>
            <FieldLabel htmlFor='value-package-reset-count-mode'>
              {t('Mode')}
            </FieldLabel>
            <NativeSelect
              id='value-package-reset-count-mode'
              value={mode}
              disabled={submitting}
              onChange={(event) =>
                setMode(
                  event.target
                    .value as OrderManagementValuePackageResetCountAdjustMode
                )
              }
            >
              <NativeSelectOption value='set'>{t('Set')}</NativeSelectOption>
              <NativeSelectOption value='add'>{t('Add')}</NativeSelectOption>
              <NativeSelectOption value='subtract'>
                {t('Subtract')}
              </NativeSelectOption>
            </NativeSelect>
          </Field>
          <Field>
            <FieldLabel htmlFor='value-package-reset-count-value'>
              {t('Reset count')}
            </FieldLabel>
            <Input
              id='value-package-reset-count-value'
              type='number'
              min={0}
              step={1}
              value={value}
              disabled={submitting}
              onChange={(event) => setValue(event.target.value)}
            />
          </Field>
          <Field>
            <FieldLabel htmlFor='value-package-reset-count-reason'>
              {t('Reason')}
            </FieldLabel>
            <Textarea
              id='value-package-reset-count-reason'
              value={reason}
              disabled={submitting}
              placeholder={t('Reason')}
              onChange={(event) => setReason(event.target.value)}
            />
          </Field>
        </FieldGroup>
        <DialogFooter>
          <Button
            type='button'
            variant='outline'
            disabled={submitting}
            onClick={() => onOpenChange(false)}
          >
            {t('Cancel')}
          </Button>
          <Button type='button' disabled={submitting} onClick={submit}>
            {t('Adjust reset count')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

const valuePackagePlanGiftKeys = {
  plans: ['order-management', 'value-package-plan-gift'] as const,
}

function PlanGiftSettingsCard() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [drafts, setDrafts] = useState<Record<number, number>>({})

  const plansQuery = useQuery({
    queryKey: valuePackagePlanGiftKeys.plans,
    queryFn: async () => {
      const result = await getAdminPlans()
      if (!result.success) {
        throw new Error(result.message || '')
      }
      return (result.data || []).filter(
        (record) => record.plan.plan_kind === 'value_package'
      )
    },
  })

  const saveMutation = useMutation({
    mutationFn: async ({
      planId,
      giftResetCount,
    }: {
      planId: number
      giftResetCount: number
    }) => patchPlanGiftResetCount(planId, giftResetCount),
    onSuccess: async (result) => {
      if (!result.success) {
        toast.error(result.message || t('Failed to update gift reset cards'))
        return
      }
      toast.success(t('Gift reset cards updated'))
      await queryClient.invalidateQueries({
        queryKey: valuePackagePlanGiftKeys.plans,
      })
    },
    onError: (error) => {
      toast.error(
        error instanceof Error
          ? error.message
          : t('Failed to update gift reset cards')
      )
    },
  })

  const records = plansQuery.data ?? []

  const renderContent = () => {
    if (plansQuery.isLoading) {
      return (
        <div className='flex flex-col gap-2'>
          <Skeleton className='h-9 w-full' />
          <Skeleton className='h-9 w-full' />
        </div>
      )
    }

    if (plansQuery.isError) {
      return (
        <Alert variant='destructive'>
          <AlertDescription className='flex flex-wrap items-center justify-between gap-2'>
            <span>
              {plansQuery.error instanceof Error && plansQuery.error.message
                ? plansQuery.error.message
                : t('Failed to load value package plans')}
            </span>
            <Button
              type='button'
              size='sm'
              variant='outline'
              onClick={() => void plansQuery.refetch()}
            >
              {t('Retry')}
            </Button>
          </AlertDescription>
        </Alert>
      )
    }

    if (records.length === 0) {
      return (
        <p className='text-muted-foreground text-sm'>
          {t(
            'No enabled day, week, or month packages are available. Enable a package plan first.'
          )}
        </p>
      )
    }

    return (
      <div className='flex flex-col gap-2'>
        {records.map(({ plan }) => {
          const currentGift = Number(plan.gift_reset_count || 0)
          const draft = drafts[plan.id] ?? currentGift
          const dirty = draft !== currentGift
          const saving =
            saveMutation.isPending && saveMutation.variables?.planId === plan.id
          return (
            <div
              key={plan.id}
              className='flex flex-wrap items-center justify-between gap-3 rounded-lg border px-3 py-2'
            >
              <div className='flex items-center gap-2'>
                <span className='font-medium'>{plan.title}</span>
                <Badge variant='secondary'>
                  {packageLabel(plan.package_type || '', t)}
                </Badge>
                {!plan.enabled && (
                  <Badge variant='outline'>{t('Disabled')}</Badge>
                )}
              </div>
              <div className='flex items-center gap-2'>
                <Input
                  className='w-24'
                  type='number'
                  min={0}
                  max={100}
                  step={1}
                  value={draft}
                  aria-label={`${plan.title}: ${t('Gift reset cards on activation')}`}
                  disabled={saveMutation.isPending}
                  onChange={(event) =>
                    setDrafts((previous) => ({
                      ...previous,
                      [plan.id]: Math.max(
                        0,
                        Math.min(
                          100,
                          Math.floor(Number(event.target.value) || 0)
                        )
                      ),
                    }))
                  }
                />
                <Button
                  type='button'
                  size='sm'
                  disabled={!dirty || saveMutation.isPending}
                  onClick={() =>
                    saveMutation.mutate({
                      planId: plan.id,
                      giftResetCount: draft,
                    })
                  }
                >
                  {saving && <Spinner data-icon='inline-start' />}
                  {t('Save')}
                </Button>
              </div>
            </div>
          )
        })}
      </div>
    )
  }

  return (
    <Card>
      <CardHeader>
        <div className='flex items-start gap-3'>
          <RotateCcw className='text-primary mt-1 size-5' aria-hidden='true' />
          <div className='flex flex-col gap-1'>
            <CardTitle>{t('Gift reset cards on activation')}</CardTitle>
            <CardDescription>
              {t(
                'Reset cards gifted to the user each time this package is purchased or redeemed (0 = none).'
              )}
            </CardDescription>
          </div>
        </div>
      </CardHeader>
      <CardContent>{renderContent()}</CardContent>
    </Card>
  )
}

export function ValuePackageManagementPage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [keywordInput, setKeywordInput] = useState('')
  const [keyword, setKeyword] = useState('')
  const [packageType, setPackageType] = useState<PackageFilter>('all')
  const [active, setActive] = useState<ActiveFilter>('active')
  const [page, setPage] = useState(1)
  const [adjustTarget, setAdjustTarget] =
    useState<OrderManagementValuePackageManagementRow | null>(null)

  const params = useMemo(
    () => ({
      keyword,
      package_type: packageType,
      active,
      page,
      page_size: PAGE_SIZE,
    }),
    [active, keyword, packageType, page]
  )

  const usersQuery = useQuery({
    queryKey: orderManagementValuePackageManagementKeys.users(params),
    queryFn: async () => {
      const result = await getOrderManagementValuePackageUsers(params)
      if (!result.success) {
        throw new Error(
          result.message || t('Failed to load value package users')
        )
      }
      return result.data || { items: [], total: 0 }
    },
    placeholderData: (previousData) => previousData,
    refetchInterval: 15_000,
  })

  const adjustMutation = useMutation({
    mutationFn: adjustOrderManagementValuePackageResetCount,
    onSuccess: async (result) => {
      if (!result.success) {
        toast.error(result.message || t('Failed to adjust reset count'))
        return
      }
      toast.success(t('Reset count updated'))
      setAdjustTarget(null)
      await queryClient.invalidateQueries({
        queryKey: ['order-management', 'value-package-management'],
      })
    },
    onError: () => {
      toast.error(t('Failed to adjust reset count'))
    },
  })

  const rows = usersQuery.data?.items ?? []
  const total = usersQuery.data?.total ?? 0
  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE))

  const applySearch = () => {
    setKeyword(keywordInput.trim())
    setPage(1)
  }

  return (
    <>
      <SectionPageLayout>
        <SectionPageLayout.Title>
          {t('Value Package Management')}
        </SectionPageLayout.Title>
        <SectionPageLayout.Actions>
          <Button
            type='button'
            variant='outline'
            size='sm'
            disabled={usersQuery.isFetching}
            onClick={() => void usersQuery.refetch()}
          >
            <RefreshCw
              data-icon='inline-start'
              className={usersQuery.isFetching ? 'animate-spin' : undefined}
            />
            {t('Refresh')}
          </Button>
        </SectionPageLayout.Actions>
        <SectionPageLayout.Content>
          <div className='mx-auto flex w-full max-w-7xl flex-col gap-4 sm:gap-5'>
            <Card>
              <CardHeader>
                <div className='flex items-start gap-3'>
                  <Sparkles className='text-primary mt-1 size-5' />
                  <div className='flex flex-col gap-1'>
                    <CardTitle>{t('Value Package Management')}</CardTitle>
                    <CardDescription>
                      {t(
                        'Manage day, week, and month package reset counts and realtime quota.'
                      )}
                    </CardDescription>
                  </div>
                </div>
              </CardHeader>
              <CardContent>
                <div className='grid gap-3 lg:grid-cols-[minmax(0,1fr)_auto_auto_auto]'>
                  <div className='flex gap-2'>
                    <Input
                      value={keywordInput}
                      placeholder={t('Search user')}
                      onChange={(event) => setKeywordInput(event.target.value)}
                      onKeyDown={(event) => {
                        if (event.key === 'Enter') applySearch()
                      }}
                    />
                    <Button
                      type='button'
                      variant='outline'
                      onClick={applySearch}
                    >
                      <Search data-icon='inline-start' />
                      {t('Search')}
                    </Button>
                  </div>
                  <NativeSelect
                    value={packageType}
                    onChange={(event) => {
                      setPackageType(event.target.value as PackageFilter)
                      setPage(1)
                    }}
                  >
                    <NativeSelectOption value='all'>
                      {t('All packages')}
                    </NativeSelectOption>
                    <NativeSelectOption value='day'>
                      {t('Day package')}
                    </NativeSelectOption>
                    <NativeSelectOption value='week'>
                      {t('Week package')}
                    </NativeSelectOption>
                    <NativeSelectOption value='month'>
                      {t('Month package')}
                    </NativeSelectOption>
                  </NativeSelect>
                  <NativeSelect
                    value={active}
                    onChange={(event) => {
                      setActive(event.target.value as ActiveFilter)
                      setPage(1)
                    }}
                  >
                    <NativeSelectOption value='active'>
                      {t('Active')}
                    </NativeSelectOption>
                    <NativeSelectOption value='expired'>
                      {t('Expired')}
                    </NativeSelectOption>
                    <NativeSelectOption value='all'>
                      {t('All')}
                    </NativeSelectOption>
                  </NativeSelect>
                  <Badge
                    variant='secondary'
                    className='h-8 justify-center px-3'
                  >
                    {t('Auto-refresh every 15 seconds')}
                  </Badge>
                </div>
              </CardContent>
            </Card>

            <PlanGiftSettingsCard />

            <Card className='min-h-[560px]'>
              <CardHeader>
                <CardTitle>{t('Value Package Management')}</CardTitle>
                <CardDescription>
                  {t('Reset count')} / {t('Quota periods')}
                </CardDescription>
              </CardHeader>
              <CardContent className='min-h-0 flex-1'>
                <ScrollArea className='h-[520px] rounded-lg border'>
                  <Table>
                    <TableHeader className='bg-background sticky top-0 z-10'>
                      <TableRow>
                        <TableHead>{t('User')}</TableHead>
                        <TableHead>{t('Package')}</TableHead>
                        <TableHead>{t('Status')}</TableHead>
                        <TableHead>{t('Reset count')}</TableHead>
                        <TableHead>{t('Quota periods')}</TableHead>
                        <TableHead>{t('Last reset')}</TableHead>
                        <TableHead>{t('Expires at')}</TableHead>
                        <TableHead className='text-right'>
                          {t('Actions')}
                        </TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {usersQuery.isLoading ? (
                        <LoadingRows />
                      ) : rows.length === 0 ? (
                        <TableRow>
                          <TableCell colSpan={8}>
                            <Empty className='border-0'>
                              <EmptyHeader>
                                <EmptyMedia variant='icon'>
                                  <Sparkles />
                                </EmptyMedia>
                                <EmptyTitle>
                                  {t('No active value package users')}
                                </EmptyTitle>
                                <EmptyDescription>
                                  {t(
                                    'Users who enable value package cards will appear here with synced 5-hour, month-card 7-day period, and package total usage.'
                                  )}
                                </EmptyDescription>
                              </EmptyHeader>
                            </Empty>
                          </TableCell>
                        </TableRow>
                      ) : (
                        rows.map((row) => {
                          const periods = getValuePackagePeriodLimits(
                            row.usage,
                            row.package_type
                          )
                          return (
                            <TableRow key={row.subscription_id}>
                              <TableCell>
                                <div className='flex min-w-36 flex-col gap-1'>
                                  <span className='font-medium'>
                                    {row.display_name ||
                                      row.username ||
                                      `#${row.user_id}`}
                                  </span>
                                  <span className='text-muted-foreground text-xs'>
                                    {row.username || '-'} · #{row.user_id}
                                  </span>
                                </div>
                              </TableCell>
                              <TableCell>
                                <div className='flex min-w-40 flex-col gap-1'>
                                  <Badge variant='secondary' className='w-fit'>
                                    {packageLabel(row.package_type, t)}
                                  </Badge>
                                  <span className='truncate font-medium'>
                                    {row.plan_title || '-'}
                                  </span>
                                </div>
                              </TableCell>
                              <TableCell>
                                <div className='flex flex-col gap-1'>
                                  <Badge
                                    variant={
                                      row.enabled ? 'default' : 'outline'
                                    }
                                  >
                                    {row.enabled ? t('Active') : t('Closed')}
                                  </Badge>
                                  <span className='text-muted-foreground text-xs'>
                                    {row.subscription_status}
                                  </span>
                                </div>
                              </TableCell>
                              <TableCell>
                                <span className='font-semibold tabular-nums'>
                                  {row.reset_count}
                                </span>
                              </TableCell>
                              <TableCell>
                                <ValuePackagePeriodList periods={periods} />
                              </TableCell>
                              <TableCell>
                                <span className='text-muted-foreground tabular-nums'>
                                  {formatResetTime(row.last_reset_at, t)}
                                </span>
                              </TableCell>
                              <TableCell>
                                <span className='text-muted-foreground tabular-nums'>
                                  {formatTimestampToDate(row.end_time)}
                                </span>
                              </TableCell>
                              <TableCell className='text-right'>
                                <Button
                                  type='button'
                                  size='sm'
                                  variant='outline'
                                  onClick={() => setAdjustTarget(row)}
                                >
                                  <RotateCcw data-icon='inline-start' />
                                  {t('Adjust reset count')}
                                </Button>
                              </TableCell>
                            </TableRow>
                          )
                        })
                      )}
                    </TableBody>
                  </Table>
                </ScrollArea>
              </CardContent>
              <CardFooter className='flex items-center justify-between gap-3'>
                <div className='text-muted-foreground text-sm'>
                  {t('Total')}: {total}
                </div>
                <Pagination className='mx-0 w-auto'>
                  <PaginationContent>
                    <PaginationItem>
                      <PaginationPrevious
                        href='#'
                        onClick={(event) => {
                          event.preventDefault()
                          setPage((current) => Math.max(1, current - 1))
                        }}
                        className={
                          page <= 1 ? 'pointer-events-none opacity-50' : ''
                        }
                      />
                    </PaginationItem>
                    <PaginationItem>
                      <span className='text-muted-foreground px-2 text-sm'>
                        {page} / {totalPages}
                      </span>
                    </PaginationItem>
                    <PaginationItem>
                      <PaginationNext
                        href='#'
                        onClick={(event) => {
                          event.preventDefault()
                          setPage((current) =>
                            Math.min(totalPages, current + 1)
                          )
                        }}
                        className={
                          page >= totalPages
                            ? 'pointer-events-none opacity-50'
                            : ''
                        }
                      />
                    </PaginationItem>
                  </PaginationContent>
                </Pagination>
              </CardFooter>
            </Card>
          </div>
        </SectionPageLayout.Content>
      </SectionPageLayout>

      <ResetCountDialog
        row={adjustTarget}
        open={adjustTarget !== null}
        submitting={adjustMutation.isPending}
        onOpenChange={(open) => !open && setAdjustTarget(null)}
        onSubmit={(request) => adjustMutation.mutate(request)}
      />
    </>
  )
}
