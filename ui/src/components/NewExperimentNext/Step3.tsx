import { Box, Button, Typography } from '@material-ui/core'
import { useStoreDispatch, useStoreSelector } from 'store'

import DoneAllIcon from '@material-ui/icons/DoneAll'
import { Experiment } from 'components/NewExperiment/types'
import Paper from 'components-mui/Paper'
import PaperTop from 'components-mui/PaperTop'
import PublishIcon from '@material-ui/icons/Publish'
import React from 'react'
import T from 'components/T'
import api from 'api'
import { parseSubmit } from 'lib/formikhelpers'
import { resetNewExperiment } from 'slices/experiments'
import { setAlert } from 'slices/globalStatus'
import { useHistory } from 'react-router-dom'
import { useIntl } from 'react-intl'

interface Step3Props {
  onSubmit?: (values: Experiment) => void
}

const Step3: React.FC<Step3Props> = ({ onSubmit }) => {
  const state = useStoreSelector((state) => state)
  const { step1, step2, basic, target } = state.experiments
  const { debugMode } = state.settings
  const dispatch = useStoreDispatch()

  const history = useHistory()
  const intl = useIntl()

  const submitExperiment = () => {
    const parsedValues = parseSubmit({
      ...basic,
      target,
    })

    if (process.env.NODE_ENV === 'development' || debugMode) {
      console.debug('Debug parsedValues:', parsedValues)
    }

    if (!debugMode) {
      if (onSubmit) {
        console.log(1)
        onSubmit(parsedValues)
      } else {
        api.experiments
          .newExperiment(parsedValues)
          .then(() => {
            dispatch(
              setAlert({
                type: 'success',
                message: intl.formatMessage({ id: 'common.createSuccessfully' }),
              })
            )

            dispatch(resetNewExperiment())

            history.push('/experiments')
          })
          .catch(console.error)
      }
    }
  }

  return (
    <>
      {step1 && step2 && (
        <Paper>
          <PaperTop title={T('common.submit')} />
          <Box p={3} textAlign="center">
            <DoneAllIcon fontSize="large" />
            <Typography>{T('newE.complete')}</Typography>
            <Box mt={6} textAlign="right">
              <Button variant="contained" color="primary" startIcon={<PublishIcon />} onClick={submitExperiment}>
                {T('common.submit')}
              </Button>
            </Box>
          </Box>
        </Paper>
      )}
    </>
  )
}

export default Step3
