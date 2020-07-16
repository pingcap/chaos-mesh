import { Box, Chip, TextField, TextFieldProps } from '@material-ui/core'
import { Field, getIn, useFormikContext } from 'formik'

import { Experiment } from 'components/NewExperiment/types'
import React from 'react'

const SelectField: React.FC<TextFieldProps & { multiple?: boolean }> = ({ multiple = false, ...props }) => {
  const { values, setFieldValue } = useFormikContext<Experiment>()

  const onDelete = (val: string) => () =>
    setFieldValue(
      props.name!,
      getIn(values, props.name!).filter((d: string) => d !== val)
    )

  const SelectProps = {
    multiple,
    renderValue: multiple
      ? (selected: any) => (
          <Box display="flex" flexWrap="wrap">
            {(selected as string[]).map((val) => (
              <Box key={val} m={0.5}>
                <Chip
                  style={{ height: 24 }}
                  label={val}
                  color="primary"
                  onDelete={onDelete(val)}
                  onMouseDown={(e) => e.stopPropagation()}
                />
              </Box>
            ))}
          </Box>
        )
      : undefined,
  }

  return (
    <Box mb={2}>
      <Field {...props} as={TextField} variant="outlined" select margin="dense" fullWidth SelectProps={SelectProps} />
    </Box>
  )
}

export default SelectField
