import ReactDOM from 'react-dom/client'
import { App as AntdApp, ConfigProvider } from 'antd'
import zhCN from 'antd/locale/zh_CN'
import App from './App'
import './index.css'

;(window as Window & { __ANT_APP_BOOTED__?: boolean }).__ANT_APP_BOOTED__ = true

ReactDOM.createRoot(document.getElementById('root')!).render(
  <ConfigProvider
    locale={zhCN}
    componentSize="small"
    theme={{
      token: {
        colorPrimary: '#1e293b',
        colorInfo: '#3b82f6',
        colorSuccess: '#22c55e',
        colorWarning: '#f59e0b',
        colorError: '#ef4444',
        colorText: '#1e293b',
        colorTextSecondary: '#475569',
        colorTextTertiary: '#64748b',
        colorBgBase: '#f8fafc',
        colorBgContainer: '#ffffff',
        colorBgElevated: '#ffffff',
        colorBorder: '#e2e8f0',
        colorBorderSecondary: '#f1f5f9',
        borderRadius: 8,
        fontSize: 13,
        fontFamily: "-apple-system, BlinkMacSystemFont, 'Segoe UI', 'Noto Sans SC', sans-serif",
        boxShadow: '0 4px 6px -1px rgb(0 0 0 / 0.07), 0 2px 4px -2px rgb(0 0 0 / 0.05)',
      },
      components: {
        Card: {
          borderRadiusLG: 12,
          paddingLG: 16,
        },
        Table: {
          headerBg: '#f8fafc',
          headerColor: '#1e293b',
          rowHoverBg: '#f8fafc',
          cellPaddingBlockSM: 8,
          cellPaddingInlineSM: 10,
        },
        Button: {
          borderRadius: 8,
          controlHeightSM: 30,
        },
        Drawer: {
          colorBgElevated: '#ffffff',
          paddingLG: 16,
        },
        Notification: {
          borderRadiusLG: 12,
        },
      },
    }}
  >
    <AntdApp>
      <App />
    </AntdApp>
  </ConfigProvider>,
)
