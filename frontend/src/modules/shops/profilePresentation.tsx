import { ReactNode } from 'react'
import { Link } from 'react-router-dom'
import { ExternalLink } from 'lucide-react'
import { Badge } from '../../shared/components'
import type { DataTableColumn } from '../../shared/components'
import { asmStatusKind, asmStatusLabel, dataCompletenessLabel, sourceLabel } from './api'
import type { ShopProfile } from './types'

export function platformLabel(platformCode: string) {
  if (!platformCode) return '-'
  if (platformCode === 'alibaba') return '1688 / Alibaba'
  return platformCode
}

export function formatProfileTime(value: string) {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return new Intl.DateTimeFormat('zh-CN', {
    dateStyle: 'medium',
    timeStyle: 'short',
  }).format(date)
}

export function asmBadge(status: string) {
  const kind = asmStatusKind(status)
  if (kind === 'connected') return <Badge variant="success">{asmStatusLabel(status)}</Badge>
  if (kind === 'error') return <Badge variant="error">{asmStatusLabel(status)}</Badge>
  return <Badge variant="warning">{asmStatusLabel(status)}</Badge>
}

export function dataCompletenessBadge(status: string) {
  if (status === 'complete') return <Badge variant="success">{dataCompletenessLabel(status)}</Badge>
  if (status === 'partial') return <Badge variant="warning">{dataCompletenessLabel(status)}</Badge>
  return <Badge variant="default">{dataCompletenessLabel(status)}</Badge>
}

export function authorizationBadge(profile: ShopProfile) {
  const label = profile.authorizationStatusLabel || profile.authorizationStatus || '未配置'
  if (profile.authorizationStatus === 'ready' || profile.authorizationStatus === 'valid') {
    return <Badge variant="success">{label}</Badge>
  }
  if (profile.authorizationStatus === 'disabled' || profile.authorizationStatus === 'revoked') {
    return <Badge variant="error">{label}</Badge>
  }
  if (profile.authorizationStatus) {
    return <Badge variant="warning">{label}</Badge>
  }
  return <Badge variant="default">{label}</Badge>
}

function compactText(value: string, width = 180) {
  return (
    <span className="block truncate text-[var(--color-text-secondary)]" style={{ maxWidth: width }} title={value || '-'}>
      {value || '-'}
    </span>
  )
}

export function buildShopProfileColumns(): DataTableColumn<ShopProfile>[] {
  return [
    {
      key: 'shopName',
      title: '店铺',
      width: 280,
      sortable: true,
      filterable: true,
      sortValue: (profile) => profile.shopName || profile.shopId,
      filterValue: (profile) => `${profile.shopName} ${profile.shopId} ${profile.fullShopName} ${profile.shopCode}`,
      render: (_, profile) => (
        <div className="flex min-w-0 flex-col gap-1">
          <Link
            className="max-w-[230px] truncate text-sm font-medium text-[var(--color-accent)] hover:underline"
            title={profile.shopName || profile.shopId}
            to={`/shops/${encodeURIComponent(profile.shopId)}`}
            onClick={(event) => event.stopPropagation()}
          >
            {profile.shopName || profile.shopId}
          </Link>
          <span className="max-w-[230px] truncate text-xs text-[var(--color-text-muted)]" title={profile.fullShopName || profile.shopId}>
            {profile.fullShopName || profile.shopId}
          </span>
        </div>
      ),
    },
    {
      key: 'shopCode',
      title: '店铺编码',
      width: 170,
      sortable: true,
      filterable: true,
      render: (value) => compactText(String(value || ''), 150),
    },
    {
      key: 'platformCode',
      title: '平台',
      width: 140,
      sortable: true,
      filterable: true,
      filterValue: (profile) => `${profile.platformCode} ${profile.platformName} ${profile.platformSubtype}`,
      render: (_, profile) => <Badge variant="default">{profile.platformName || platformLabel(profile.platformCode)}</Badge>,
    },
    {
      key: 'asmStatus',
      title: 'ASM 状态',
      width: 140,
      sortable: true,
      filterable: true,
      filterValue: (profile) => asmStatusLabel(profile.asmStatus),
      render: (value) => asmBadge(String(value || '')),
    },
    {
      key: 'authorizationStatus',
      title: '授权状态',
      width: 140,
      sortable: true,
      filterable: true,
      filterValue: (profile) => profile.authorizationStatusLabel || profile.authorizationStatus,
      render: (_, profile) => authorizationBadge(profile),
    },
    {
      key: 'operatorName',
      title: '运营',
      width: 140,
      sortable: true,
      filterable: true,
      filterValue: (profile) => `${profile.operatorName} ${profile.operatorUsername}`,
      render: (_, profile) => compactText(profile.operatorName || profile.operatorUsername, 120),
    },
    {
      key: 'businessManagerName',
      title: '业务经理',
      width: 150,
      sortable: true,
      filterable: true,
      filterValue: (profile) => `${profile.businessManagerName} ${profile.businessManagerUsername}`,
      render: (_, profile) => compactText(profile.businessManagerName || profile.businessManagerUsername, 130),
    },
    {
      key: 'department',
      title: '部门',
      width: 150,
      sortable: true,
      filterable: true,
      render: (value) => compactText(String(value || ''), 130),
    },
    {
      key: 'brandName',
      title: '品牌',
      width: 150,
      sortable: true,
      filterable: true,
      render: (value) => compactText(String(value || ''), 130),
    },
    {
      key: 'mainCategory',
      title: '主营类目',
      width: 180,
      sortable: true,
      filterable: true,
      render: (value) => compactText(String(value || ''), 160),
    },
    {
      key: 'lastSyncedAt',
      title: '最近同步',
      width: 180,
      sortable: true,
      sortValue: (profile) => Date.parse(profile.lastSyncedAt || '') || 0,
      render: (value) => (
        <span className="block max-w-[160px] truncate text-xs text-[var(--color-text-muted)]" title={String(value || '-')}>
          {formatProfileTime(String(value || ''))}
        </span>
      ),
    },
  ]
}

export type DetailField = {
  label: string
  value: ReactNode
}

export type DetailGroup = {
  title: string
  subtitle: string
  fields: DetailField[]
}

export function buildShopProfileDetailGroups(profile: ShopProfile): DetailGroup[] {
  return [
    {
      title: '基础资料',
      subtitle: 'ASM 店铺业务主数据',
      fields: [
        { label: '店铺名称', value: profile.shopName },
        { label: 'ASM Shop ID', value: profile.asmShopId },
        { label: 'Shop ID', value: profile.shopId },
        { label: '店铺编码', value: profile.shopCode },
        { label: '店铺别名', value: profile.shopAlias },
        { label: '完整店铺名', value: profile.fullShopName },
        { label: '平台', value: profile.platformName || platformLabel(profile.platformCode) },
        { label: '平台子类型', value: profile.platformSubtype },
        { label: '主营类目', value: profile.mainCategory },
      ],
    },
    {
      title: '经营归属',
      subtitle: '客户端只展示运营识别需要的归属字段',
      fields: [
        { label: '负责人', value: profile.ownerName },
        { label: '运营', value: profile.operatorName },
        { label: '运营账号', value: profile.operatorUsername },
        { label: '业务经理', value: profile.businessManagerName },
        { label: '业务经理账号', value: profile.businessManagerUsername },
        { label: '部门', value: profile.department },
        { label: '分公司', value: profile.subCompanyName },
      ],
    },
    {
      title: '联系与品牌',
      subtitle: 'ASM 店铺扩展资料',
      fields: [
        {
          label: '店铺地址',
          value: profile.shopUrl ? (
            <a className="inline-flex items-center gap-1 text-[var(--color-accent)] hover:underline" href={profile.shopUrl} target="_blank" rel="noreferrer">
              {profile.shopUrl}
              <ExternalLink className="h-3.5 w-3.5" />
            </a>
          ) : '',
        },
        { label: '邮箱', value: profile.shopEmail },
        { label: '电话', value: profile.shopPhone },
        { label: '品牌', value: profile.brandName },
        { label: '高级会员', value: profile.advancedMemberName },
      ],
    },
    {
      title: '状态与同步',
      subtitle: 'ASM 资料状态与本地授权状态',
      fields: [
        { label: 'ASM 状态', value: asmBadge(profile.asmStatus) },
        { label: '授权状态', value: authorizationBadge(profile) },
        { label: '资料完整度', value: dataCompletenessBadge(profile.dataCompleteness) },
        { label: '最近同步', value: formatProfileTime(profile.lastSyncedAt) },
        { label: '数据来源', value: sourceLabel(profile.source) },
      ],
    },
  ]
}
