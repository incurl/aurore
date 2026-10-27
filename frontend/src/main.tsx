import { ChakraProvider } from '@chakra-ui/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import React from 'react';
import ReactDOM from 'react-dom/client';

import './i18n';

import App from './App';
import { system } from './wrapped-invoke-ui/platform/ui/theme/system';

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      // The Wails event stream pushes job updates; the refetch interval
      // is only a safety net, so retries stay off.
      retry: 1,
      refetchOnWindowFocus: false,
    },
  },
});

ReactDOM.createRoot(document.getElementById('root') as HTMLElement).render(
  <React.StrictMode>
    <QueryClientProvider client={queryClient}>
      <ChakraProvider value={system}>
        <App />
      </ChakraProvider>
    </QueryClientProvider>
  </React.StrictMode>,
);
