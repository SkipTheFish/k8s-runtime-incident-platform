import axios from "axios";

const baseURL = import.meta.env.VITE_API_BASE_URL || "/api/v1";
const tenantId = import.meta.env.VITE_TENANT_ID || "t1";

export const demoMode = import.meta.env.VITE_DEMO_MODE === "true";

export const client = axios.create({
  baseURL,
  timeout: 10000,
  headers: {
    "X-Tenant-ID": tenantId,
  },
});