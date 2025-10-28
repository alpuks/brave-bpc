import {
  Button,
  Modal,
  ModalBody,
  ModalContent,
  ModalFooter,
  ModalHeader,
  Textarea,
  addToast,
  useDisclosure,
} from "@heroui/react";
import { createFileRoute } from "@tanstack/react-router";
import { useCallback, useEffect, useMemo, useState } from "react";

export const Route = createFileRoute("/_auth/admin")({
  component: RouteComponent,
});

const CONFIG_ENDPOINT = "/api/admin/index-page";

interface IndexConfigResponse {
  content?: string;
}

function RouteComponent() {
  const { isOpen, onOpen, onOpenChange, onClose } = useDisclosure();
  const [formValue, setFormValue] = useState<string>("");
  const [isLoading, setIsLoading] = useState(false);
  const [isSaving, setIsSaving] = useState(false);

  const modalHelpText = useMemo(
    () =>
      "Provide HTML content that should appear on the public index page. This will be submitted to the admin configuration endpoint.",
    []
  );

  useEffect(() => {
    if (!isOpen) {
      return;
    }

    const fetchConfig = async () => {
      setIsLoading(true);
      try {
        const response = await fetch(CONFIG_ENDPOINT, {
          method: "GET",
          credentials: "include",
        });

        if (!response.ok) {
          throw new Error(`Failed to load index configuration (${response.status})`);
        }

        const data: IndexConfigResponse = await response.json().catch(() => ({}));
        setFormValue(data.content ?? "");
      } catch (error) {
        addToast({
          title: "Configuration",
          description: error instanceof Error ? error.message : "Unable to load current configuration.",
          color: "danger",
        });
      } finally {
        setIsLoading(false);
      }
    };

    void fetchConfig();
  }, [isOpen]);

  const handleSave = useCallback(async () => {
    setIsSaving(true);
    try {
      const response = await fetch(CONFIG_ENDPOINT, {
        method: "POST",
        credentials: "include",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({ content: formValue }),
      });

      if (!response.ok) {
        throw new Error(`Failed to save configuration (${response.status})`);
      }

      addToast({
        title: "Configuration saved",
        description: "Index page content was updated successfully.",
        color: "success",
      });
      onClose();
    } catch (error) {
      addToast({
        title: "Save failed",
        description: error instanceof Error ? error.message : "Unable to save index configuration.",
        color: "danger",
      });
    } finally {
      setIsSaving(false);
    }
  }, [formValue, onClose]);

  return (
    <div className="flex w-full flex-col gap-6">
      <header className="flex flex-col gap-1">
        <h1 className="text-2xl font-semibold text-default-900">Administration</h1>
        <p className="text-sm text-default-500">
          Manage site-wide settings and landing page content.
        </p>
      </header>

      <section>
        <Button onPress={onOpen} color="primary" variant="solid">
          Configure Index Page
        </Button>
      </section>

      <Modal isOpen={isOpen} onOpenChange={onOpenChange} size="xl">
        <ModalContent>
          {() => (
            <>
              <ModalHeader className="flex flex-col gap-1">
                Update Index Page Content
                <span className="text-sm font-normal text-default-500">
                  {modalHelpText}
                </span>
              </ModalHeader>
              <ModalBody>
                <Textarea
                  aria-label="Index page HTML"
                  isDisabled={isLoading}
                  minRows={12}
                  placeholder="<section> ... </section>"
                  value={formValue}
                  onValueChange={setFormValue}
                />
              </ModalBody>
              <ModalFooter>
                <Button variant="light" onPress={onClose} isDisabled={isSaving}>
                  Cancel
                </Button>
                <Button color="primary" isLoading={isSaving} onPress={() => void handleSave()}>
                  Save Changes
                </Button>
              </ModalFooter>
            </>
          )}
        </ModalContent>
      </Modal>
    </div>
  );
}
