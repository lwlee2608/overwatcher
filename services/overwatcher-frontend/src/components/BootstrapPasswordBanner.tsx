import { useState } from "react";
import { useAuth } from "../auth/context";
import { ChangePasswordModal } from "./ChangePasswordModal";

export function BootstrapPasswordBanner() {
  const { user } = useAuth();
  const [showModal, setShowModal] = useState(false);

  if (!user?.must_change_password) return null;

  return (
    <>
      <div className="border-b border-amber-200 bg-amber-50 dark:border-amber-900 dark:bg-amber-900/20">
        <div className="max-w-4xl mx-auto px-6 py-2 flex items-center justify-between gap-4">
          <p className="text-sm text-amber-800 dark:text-amber-200">
            You're signed in with the bootstrap password. Change it before
            anyone else gets to.
          </p>
          <button
            onClick={() => setShowModal(true)}
            className="shrink-0 rounded-md bg-amber-600 px-3 py-1 text-sm font-medium text-white hover:bg-amber-700"
          >
            Change password
          </button>
        </div>
      </div>
      {showModal && <ChangePasswordModal onClose={() => setShowModal(false)} />}
    </>
  );
}
