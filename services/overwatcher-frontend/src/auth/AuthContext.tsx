import { useCallback, useEffect, useMemo, useState } from "react";
import type { ReactNode } from "react";
import {
  changePassword as apiChangePassword,
  fetchMe,
  login as apiLogin,
  logout as apiLogout,
} from "../api/auth";
import type { ChangePasswordRequest, MeResponse } from "../types/auth";
import { AuthContext } from "./context";
import type { AuthContextValue } from "./context";

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<MeResponse | null>(null);
  const [loading, setLoading] = useState(true);

  const refresh = useCallback(async () => {
    const me = await fetchMe();
    setUser(me);
  }, []);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const me = await fetchMe();
        if (!cancelled) setUser(me);
      } catch {
        if (!cancelled) setUser(null);
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  const login = useCallback(async (email: string, password: string) => {
    const me = await apiLogin({ email, password });
    setUser(me);
    return me;
  }, []);

  const logout = useCallback(async () => {
    await apiLogout();
    setUser(null);
  }, []);

  const changePassword = useCallback(async (req: ChangePasswordRequest) => {
    await apiChangePassword(req);
    // ChangePassword revokes all sessions server-side, so the next /auth/me
    // would 401 and redirect anyway. Clear locally so the UI doesn't flicker.
    setUser(null);
  }, []);

  const value = useMemo<AuthContextValue>(
    () => ({ user, loading, login, logout, refresh, changePassword }),
    [user, loading, login, logout, refresh, changePassword]
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}
