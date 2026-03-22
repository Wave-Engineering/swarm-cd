import { Alert, AlertDescription, AlertIcon, AlertTitle, CloseButton, Box, VStack } from "@chakra-ui/react"
import React, { useState } from "react"

function ConfigWarningBanner({ warnings }: Readonly<{ warnings: string[] }>): React.ReactElement | null {
  const [dismissed, setDismissed] = useState(false)

  if (dismissed || warnings.length === 0) {
    return null
  }

  return (
    <Alert status="warning" mb={4} borderRadius="md" alignItems="flex-start">
      <AlertIcon />
      <Box flex="1">
        <AlertTitle>Configuration Warning{warnings.length > 1 ? "s" : ""}</AlertTitle>
        <AlertDescription>
          <VStack align="start" spacing={1}>
            {warnings.map((warning, index) => (
              <Box key={index}>{warning}</Box>
            ))}
          </VStack>
        </AlertDescription>
      </Box>
      <CloseButton
        alignSelf="flex-start"
        position="relative"
        right={-1}
        top={-1}
        onClick={() => setDismissed(true)}
        aria-label="Dismiss warning"
      />
    </Alert>
  )
}

export default ConfigWarningBanner
