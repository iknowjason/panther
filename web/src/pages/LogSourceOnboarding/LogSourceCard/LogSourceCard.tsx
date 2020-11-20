/**
 * Copyright (C) 2020 Panther Labs Inc
 *
 * Panther Enterprise is licensed under the terms of a commercial license available from
 * Panther Labs Inc ("Panther Commercial License") by contacting contact@runpanther.com.
 * All use, distribution, and/or modification of this software, whether commercial or non-commercial,
 * falls under the Panther Commercial License to the extent it is permitted.
 */

import * as React from 'react';
import { Box, Flex, Icon, Img, FadeIn } from 'pouncejs';
import { Link as RRLink } from 'react-router-dom';
import PantherEnterpriseLogo from 'Assets/panther-enterprise-minimal-logo.svg';
import { slugify } from 'Helpers/utils';
import useHover from 'Hooks/useHover';

interface ItemCardProps {
  logo: string;
  title: string;
  disabled?: boolean;
  to: string;
}

const LogSourceCard: React.FC<ItemCardProps> = ({ logo, title, to, disabled }) => {
  const { isHovering, handlers: hoverHandlers } = useHover();
  const titleId = slugify(title);

  const content = (
    <Box
      {...hoverHandlers}
      aria-disabled={disabled}
      border="1px solid"
      borderRadius="medium"
      transition="all 0.15s ease-in-out"
      backgroundColor={isHovering ? 'navyblue-500' : 'transparent'}
      borderColor={isHovering ? 'navyblue-500' : 'navyblue-300'}
      _focus={{ backgroundColor: 'navyblue-500', borderColor: 'navyblue-500' }}
    >
      <Flex alignItems="center" py={3} px={3}>
        <Img
          aria-labelledby={titleId}
          src={logo}
          alt={title}
          objectFit="contain"
          nativeHeight={26}
          nativeWidth={26}
        />
        <Box id={titleId} px={3} textAlign="left">
          {title}
        </Box>
        <Flex align="center" ml="auto">
          {disabled && (
            <Img
              nativeWidth={20}
              nativeHeight={20}
              alt="Panther Enterprise Logo"
              src={PantherEnterpriseLogo}
            />
          )}
          {isHovering && (
            <FadeIn from="left" offset={3}>
              <Icon type="arrow-forward" />
            </FadeIn>
          )}
        </Flex>
      </Flex>
    </Box>
  );

  if (disabled) {
    return content;
  }

  return <RRLink to={to}>{content}</RRLink>;
};

export default React.memo(LogSourceCard);
